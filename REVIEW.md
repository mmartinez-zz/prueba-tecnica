# Code Review — Task Manager Application

## Resumen Ejecutivo

El proyecto implementa una aplicación básica de gestión de tareas con backend en Go y frontend en Next.js. La arquitectura es monolítica simple, adecuada para una aplicación pequeña, pero presenta múltiples vulnerabilidades críticas de seguridad y problemas de calidad que impiden su uso en producción. Se requieren correcciones inmediatas en autenticación, validación de datos y control de concurrencia antes de cualquier despliegue.

Los 3 problemas más críticos identificados son:

1. **JWT mal construido**: Claim `exp` seteado como string en lugar de número, invalidando validación de expiración.
2. **Credenciales hardcodeadas**: Secretos JWT y credenciales de BD tienen valores por defecto inseguros.
3. **Ausencia de transacciones**: Operaciones multi-paso pueden dejar la BD en estado inconsistente.

## Arquitectura General

### Evaluación de la Arquitectura

- **Tipo de aplicación**: Gestión de tareas colaborativa con autenticación.
- **Arquitectura actual**: Monolítica con separación clara de responsabilidades (handler, model, db).
- **Adecuación**: Básica y funcional para prototipos, pero insuficiente para producción con múltiples usuarios concurrentes.
- **Despliegue**: No incluye Docker Compose; requiere configuración manual de BD y servicios separados.
- **Escalabilidad**: Limitada; no soporta horizontal scaling sin refactorización significativa.

### Recomendaciones Arquitecturales

Para producción con 10,000 usuarios:
- Agregar caché (Redis) para queries frecuentes y sesiones.
- Implementar timeouts y rate limiting para proteger contra DoS.
- Mejorar logging y monitoring para debugging.
- Optimizar queries N+1 y agregar índices si necesario.

## Problemas por Severidad

### Crítico

#### 1. JWT mal construido (claim exp como string)
**Archivo**: `backend/internal/handler/tasks.go:546`
**Problema**: En `LoginHandler`, `exp` se setea como string: `"exp": fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())`. `jwt.Parse` con `MapClaims` solo valida expiración si `exp` es numérico (float64/int64). Como string, no se valida, tokens nunca expiran realmente.
**Impacto**: Seguridad comprometida; tokens expirados aceptados, acceso no autorizado perpetuo.
**Solución**: Cambiar a `"exp": time.Now().Add(24*time.Hour).Unix()`, o mejor usar `jwt.RegisteredClaims` con `ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour))`.

#### 2. Credenciales hardcodeadas
**Archivos**: `backend/internal/middleware/auth.go:21`, `backend/internal/db/connection.go:16-18`
**Problema**: JWT secret y credenciales de BD tienen valores por defecto ("default-secret-change-in-production", "assessment").
**Impacto**: Compilación con credenciales inseguras facilita ataques si no se configuran variables de entorno.
**Solución**: Remover defaults; fallar si variables no están configuradas. Usar secrets management (Vault, AWS Secrets).

#### 3. Ausencia de control transaccional
**Archivos**: `backend/internal/handler/tasks.go:325-341`
**Problema**: Operaciones como `UpdateTask` ejecutan múltiples queries sin transacción; fallo parcial deja BD inconsistente.
**Impacto**: Actualización de tarea puede fallar dejando historial parcial.
**Solución**: Envolver operaciones multi-paso en transacciones usando `pgxpool.Begin`.

#### 4. Ausencia de timeouts en conexiones
**Archivos**: `backend/internal/db/connection.go`, `backend/cmd/server/main.go`
**Problema**: No timeouts configurados en pool de BD ni en servidor HTTP.
**Impacto**: Queries lentas pueden bloquear indefinidamente, causando DoS.
**Solución**: Configurar `MaxConnLifetime`, `MaxConnIdleTime` en pool; agregar timeouts HTTP con middleware.

#### 5. Errores de scan ignorados silenciosamente
**Archivo**: `backend/internal/handler/tasks.go:98-99`
**Problema**: En carga de tags para tareas, errores de `rows.Scan` son ignorados con `_`; fallos ocultos.
**Impacto**: Datos de tags corruptos o faltantes sin detección; aplicación parece funcionar pero con datos incompletos.
**Solución**: Manejar errores de scan; loggear y retornar error HTTP si falla.

#### 6. Errores en Exec ignorados en UpdateTask
**Archivo**: `backend/internal/handler/tasks.go:325,337`
**Problema**: En `UpdateTask`, `db.Pool.Exec` para UPDATE e INSERT edit_history ignoran errores con `_`; fallos silenciosos.
**Impacto**: Updates fallidos no detectados, BD inconsistente, usuario ve tarea "actualizada" incorrectamente.
**Solución**: Manejar errores de Exec; retornar 500 si falla.

### Alto

#### 6. Violación del principio de responsabilidad única (SRP)
**Archivo**: `backend/internal/handler/tasks.go`
**Problema**: Handlers contienen lógica de negocio, validación y acceso a datos.
**Impacto**: Código difícil de mantener y testear.
**Solución**: Extraer lógica a servicios separados (ej: `TaskService`); handlers solo manejan HTTP.

#### 7. Lógica de negocio en handlers
**Archivos**: `backend/internal/handler/tasks.go:186-235` (CreateTask)
**Problema**: Validación de campos y transformación de datos en handlers.
**Impacto**: Duplicación de código; difícil reutilizar lógica.
**Solución**: Crear structs de validación y servicios de dominio.

#### 8. Errores no manejados
**Archivos**: `backend/internal/handler/tasks.go:78-83`, múltiples lugares con `_ = db.QueryRow`
**Problema**: Errores de BD ignorados; aplicación continúa con datos potencialmente corruptos.
**Impacto**: Silencioso fallo de queries; usuario ve datos incompletos.
**Solución**: Manejar todos errores; retornar códigos HTTP apropiados (500 para BD, 404 para not found).

#### 9. Falta de propagación de contexto con timeouts
**Archivo**: `backend/internal/handler/tasks.go`
**Problema**: Context se pasa, pero sin deadlines; queries pueden bloquear indefinidamente.
**Impacto**: Recursos consumidos por queries lentas.
**Solución**: Usar `context.WithTimeout` en handlers; propagar a todas operaciones.

#### 10. Problema N+1 en queries de listado
**Archivo**: `backend/internal/handler/tasks.go:86-87`
**Problema**: Para cada tarea en `ListTasks`, se ejecuta query separada para cargar tags; con N tareas, N+1 queries.
**Impacto**: Performance pobre con muchas tareas; queries innecesarias sobrecargan BD.
**Solución**: Usar LEFT JOIN con agregación de arrays para cargar tags en una sola query.

### Medio

#### 10. Falta de tests
**Archivo**: `backend/tests/tasks_test.go` (vacío)
**Problema**: No hay tests unitarios ni de integración; solo estructura.
**Impacto**: Cambios rompen funcionalidad sin detección.
**Solución**: Implementar tests con `testify`; cubrir handlers, servicios y BD con testcontainers.

#### 11. Acoplamiento innecesario
**Archivo**: `backend/internal/handler/tasks.go:46`
**Problema**: Handlers acceden directamente a `db.Pool`.
**Impacto**: Difícil mockear en tests; cambios en BD afectan handlers.
**Solución**: Inyección de dependencias; crear interfaces para repositorios.

#### 12. Nombres inconsistentes
**Archivos**: Varios
**Problema**: Mezcla de camelCase y snake_case en algunos lugares; campos JSON vs DB.
**Impacto**: Confusión en mantenimiento.
**Solución**: Estandarizar naming conventions (snake_case para DB, camelCase para JSON).

### Bajo

#### 13. Logging pobre
**Archivos**: `backend/internal/handler/tasks.go:63`
**Problema**: Usa `log.Printf` básico; no estructurado ni con niveles.
**Impacto**: Difícil debugging en producción.
**Solución**: Usar logger estructurado como `slog` o `zap`; incluir request IDs y context.

#### 14. Organización de archivos
**Archivo**: `backend/internal/handler/tasks.go`
**Problema**: Archivo de 566 líneas con múltiples responsabilidades.
**Impacto**: Difícil navegación y mantenimiento.
**Solución**: Separar en archivos por funcionalidad (tasks.go, auth.go, etc. ya existen, pero subdividir más).

#### 15. Frontend minimalista
**Archivos**: `frontend/`
**Problema**: Sin framework UI, CSS custom, gestión de estado; solo Next.js básico.
**Impacto**: UX pobre; no escalable para features complejas.
**Solución**: Agregar shadcn/ui, TailwindCSS, Zustand para estado.

#### 16. Falta de validación en BD
**Archivo**: `database/init.sql`
**Problema**: No constraints CHECK en status/priority; dependencias de FK correctas pero insuficientes.
**Impacto**: Datos inválidos pueden insertarse.
**Solución**: Agregar CHECK constraints y triggers para validación.

#### 17. Arquitectura de despliegue ausente
**Problema**: No Docker Compose ni Dockerfiles; despliegue manual.
**Impacto**: Difícil reproducir entornos; no CI/CD.
**Solución**: Crear docker-compose.yml con servicios para BD, backend, frontend; multi-stage builds.
