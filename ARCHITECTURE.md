# Arquitectura y Decisiones

## 1. Code Review — Resumen Ejecutivo

¿Cuáles son los 2-3 problemas más graves y por qué los priorizaste?

Manejo inseguro de JWT y secreto por defecto
El middleware original permitía un JWT_SECRET por defecto en caso de no estar definido. Esto es crítico porque habilita tokens firmados con un secreto conocido en entornos no controlados.Se corrigió forzando la presencia de la variable de entorno y validando explícitamente el algoritmo HS256

Falta de integridad transaccional en operaciones críticas
El UpdateTask originalmente no garantizaba atomicidad entre la actualización de la tarea y el registro en edit_history. Esto podía generar inconsistencias (por ejemplo, task actualizada sin historial). Se implementó transacción explícita con rollback seguro y manejo correcto de errores

Manejo débil de errores en múltiples queries.
Existían varios _ = rows.Scan(...) y consultas sin validación de error. Esto puede ocultar fallos de base de datos y degradar silenciosamente la integridad del sistema. Se priorizó eliminar estos patrones en operaciones críticas


## 2. Integración de IA

- ¿Qué modelo/provider elegiste y por qué?

Se implementó un cliente para OpenAI utilizando la API moderna /v1/responses con el modelo gpt-4.1-mini.

Porque?

API más reciente y unificada
Soporte nativo para response_format: json_object
Mejor control estructural de salida
Modelo más económico que GPT-4 completo, pero más robusto que 3.5


- ¿Cómo diseñaste el prompt? ¿Qué iteraciones hiciste?

Se diseñó un prompt que:

Define el rol del modelo
Impone una estructura JSON estricta
Limita explícitamente los valores permitidos
Solicita “Return ONLY valid JSON”

- ¿Cómo manejas los casos donde el LLM falla, tarda, o retorna datos inválidos?
Actualmente:

Timeout HTTP explícito (15s)
Manejo explícito de 429 Too Many Requests
Error tipado ErrRateLimited
Validación de tags no vacío
Validación de summary no vacío

En producción agregaría:

Timeout configurable, retry con backoff exponencial, circuit breaker y logging estructurado

- ¿Cómo manejarías el costo en producción? (caching, rate limiting, batch processing)

Actualmente se utiliza gpt-4.1-mini, modelo optimizado en costo.
Se podría:

Limitar tamaño de prompt
Truncar description
Implementar cache por hash
Implementar cola asíncrona para clasificación masiva

- Si tuvieras que clasificar 10,000 tareas existentes, ¿cómo lo harías?

No lo haría a través de un endpoint

Implementaría:

Job offline
Worker queue
Procesamiento en lotes con control de rate limit
Métricas y reintentos por fallo

## 3. Docker y Orquestación

¿Qué decisiones tomaste? (multi-stage builds, networking, volumes, health checks, etc.)


Multi-stage build para reducir tamaño de imagen
Imagen runtime Alpine minima
Usuario no-root
Volumen persistente para datos
Red interna dedicada
depends_on con condición de health

Pendiente opcional:
Agregar seed automático ejecutando database/init.sql como init script del contenedor

## 4. Arquitectura para Producción

Si este proyecto fuera a producción con 10,000 usuarios concurrentes:

- ¿Qué cambiarías en la arquitectura?

Separar API y workers
Agregar cache (Redis)
Agregar queue (RabbitMQ / SQS)
Horizontal scaling del API
Pool tuning en pgx
Load balancer

- ¿Qué servicios agregarías? (cache, queue, CDN, etc.)

Redis (cache y rate limiting)
Message Queue
CDN para frontend
Centralized logging

¿Cómo manejarías el deployment?

CI/CD pipeline
Docker registry
Deploy en Kubernetes o ECS
Rolling updates
Secrets manager

- ¿Cómo manejarías el deployment?
- Incluye un diagrama si te parece útil (ASCII, Mermaid, o imagen)

          ┌──────────────┐
          │    CDN       │
          └──────┬───────┘
                 │
          ┌──────▼───────┐
          │   Frontend   │
          └──────┬───────┘
                 │
         ┌───────▼────────┐
         │  Load Balancer  │
         └───────┬────────┘
                 │
        ┌────────▼─────────┐
        │   API Instances  │
        └────────┬─────────┘
                 │
        ┌────────▼─────────┐
        │      Redis       │
        └────────┬─────────┘
                 │
        ┌────────▼─────────┐
        │   PostgreSQL     │
        └──────────────────┘


## 5. Trade-offs

¿Qué decisiones tomaste donde había más de una opción válida?

OpenAI vs Anthropic
Se eligió OpenAI por rapidez de integración. Diseño desacoplado permite cambiarlo.

Sincrónico vs Asíncrono
Se implementó sincrónico para simplificar evaluación técnica.
En producción debería ser asíncrono.

JSON parsing estricto
Se decidió validar en backend en vez de confiar en el LLM.



## 6. Qué Mejorarías con Más Tiempo

Sé específico y prioriza.

Validación completa de errores en todos los handlers
Refactor para eliminar lógica duplicada de carga de task
Centralizar respuesta JSON
Middleware para logging estructurado
Tests unitarios para service.ClassifyTask
Timeout configurable en cliente OpenAI
Observabilidad (metrics, tracing)
Seed automático en docker-compose
Rate limiting por usuario
Eliminación total de patrones _ =

## 7. Uso de IA (como herramienta de desarrollo)

¿Usaste IA para desarrollar? ¿Para qué? ¿Modificaste lo que te sugirió?

Sí.

Se utilizó IA para:

Refactor de middleware JWT
Diseño del cliente LLM
Estructuración de transacciones
Generación base de Dockerfile
Generación base de docker-compose
Iteración sobre el prompt

Todas las sugerencias fueron revisadas manualmente.
Se eliminaron defaults inseguros, se agregaron validaciones explícitas y se ajustó la lógica transaccional para cumplir criterios de consistencia

La IA se utilizó como acelerador, no como sustituto de criterio técnico