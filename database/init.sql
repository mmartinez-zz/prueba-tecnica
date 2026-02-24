-- Task Manager Database Schema
-- PostgreSQL 16

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================
-- TABLES
-- ============================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- 'admin', 'member'
    avatar_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'todo', -- 'todo', 'in_progress', 'review', 'done'
    priority VARCHAR(20) NOT NULL DEFAULT 'medium', -- 'low', 'medium', 'high', 'urgent'
    category VARCHAR(100),
    summary TEXT,
    creator_id UUID NOT NULL REFERENCES users(id),
    assignee_id UUID REFERENCES users(id),
    due_date TIMESTAMP WITH TIME ZONE,
    estimated_hours DECIMAL(5,2),
    actual_hours DECIMAL(5,2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE edit_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    field_name VARCHAR(100) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    edited_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    color VARCHAR(7) DEFAULT '#6B7280', -- hex color
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE task_tags (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    assigned_by VARCHAR(50) DEFAULT 'manual', -- 'manual', 'ai'
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (task_id, tag_id)
);

-- ============================================
-- INDEXES
-- ============================================

CREATE INDEX idx_tasks_creator ON tasks(creator_id);
CREATE INDEX idx_tasks_assignee ON tasks(assignee_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_due_date ON tasks(due_date);
CREATE INDEX idx_edit_history_task ON edit_history(task_id);
CREATE INDEX idx_task_tags_task ON task_tags(task_id);
CREATE INDEX idx_task_tags_tag ON task_tags(tag_id);

-- ============================================
-- SEED DATA
-- ============================================

-- Users (passwords are bcrypt hash of "password123")
INSERT INTO users (id, email, name, password_hash, role) VALUES
    ('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'carlos@kemeny.studio', 'Carlos Méndez', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin'),
    ('b2c3d4e5-f6a7-8901-bcde-f12345678901', 'lucia@kemeny.studio', 'Lucía Fernández', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'member'),
    ('c3d4e5f6-a7b8-9012-cdef-123456789012', 'mateo@kemeny.studio', 'Mateo Ruiz', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'member'),
    ('d4e5f6a7-b8c9-0123-defa-234567890123', 'valentina@kemeny.studio', 'Valentina López', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'member');

-- Tasks
INSERT INTO tasks (id, title, description, status, priority, creator_id, assignee_id, due_date, estimated_hours) VALUES
    ('11111111-1111-1111-1111-111111111111',
     'Implementar autenticación OAuth con Google',
     'Necesitamos agregar login con Google como alternativa al email/password. Debe soportar el flujo completo: redirect a Google, callback, creación de usuario si no existe, y linkeo si el email ya está registrado. Considerar refresh tokens.',
     'in_progress', 'high',
     'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
     NOW() + INTERVAL '5 days', 16),

    ('22222222-2222-2222-2222-222222222222',
     'Fix: Dashboard muestra datos incorrectos al filtrar por fecha',
     'Cuando el usuario filtra tareas por rango de fechas en el dashboard, los contadores de "tareas completadas" y "tareas pendientes" no se actualizan. Parece que el query del resumen no aplica el mismo filtro WHERE. Reproducible en producción.',
     'todo', 'urgent',
     'b2c3d4e5-f6a7-8901-bcde-f12345678901', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
     NOW() + INTERVAL '1 day', 4),

    ('33333333-3333-3333-3333-333333333333',
     'Migrar estilos de CSS modules a Tailwind',
     'El proyecto actualmente usa CSS modules para los componentes principales. Queremos migrar a Tailwind CSS para consistencia con el design system nuevo. Empezar por los componentes más usados: TaskCard, TaskBoard, Dashboard. No romper el layout responsive existente.',
     'todo', 'medium',
     'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'c3d4e5f6-a7b8-9012-cdef-123456789012',
     NOW() + INTERVAL '10 days', 24),

    ('44444444-4444-4444-4444-444444444444',
     'Agregar tests de integración para el API de tareas',
     'Necesitamos tests de integración que cubran: creación de tarea, actualización, borrado, listado con filtros, y asignación. Usar testcontainers para PostgreSQL. Cubrir al menos los happy paths y los errores de validación más comunes.',
     'review', 'medium',
     'c3d4e5f6-a7b8-9012-cdef-123456789012', 'c3d4e5f6-a7b8-9012-cdef-123456789012',
     NOW() + INTERVAL '3 days', 12),

    ('55555555-5555-5555-5555-555555555555',
     'Investigar opciones de rate limiting para el API',
     'Con el crecimiento de usuarios necesitamos rate limiting. Investigar: token bucket vs sliding window, implementación a nivel de middleware vs API gateway, persistencia en Redis vs in-memory. Documentar pros/cons y hacer una recomendación.',
     'todo', 'low',
     'a1b2c3d4-e5f6-7890-abcd-ef1234567890', NULL,
     NOW() + INTERVAL '15 days', 8),

    ('66666666-6666-6666-6666-666666666666',
     'Optimizar queries del listado de tareas',
     'El endpoint GET /api/tasks se está volviendo lento con +1000 tareas. Profile muestra N+1 en la carga de usuarios asignados. Implementar eager loading o JOIN. También considerar paginación cursor-based en vez de offset.',
     'in_progress', 'high',
     'b2c3d4e5-f6a7-8901-bcde-f12345678901', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
     NOW() + INTERVAL '2 days', 6),

    ('77777777-7777-7777-7777-777777777777',
     'Configurar CI/CD pipeline con GitHub Actions',
     'Necesitamos un pipeline que: ejecute tests, haga build de Docker images, y deploye a staging automáticamente en merge a main. Usar GitHub Actions. Incluir: lint, tests unitarios, tests de integración, build de imagen, push a registry, deploy a staging.',
     'todo', 'high',
     'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
     NOW() + INTERVAL '7 days', 10),

    ('88888888-8888-8888-8888-888888888888',
     'Agregar notificaciones por email cuando se acerca el deadline',
     'Los usuarios deben recibir un email 24 horas antes del due_date de sus tareas asignadas. Usar un cron job o scheduler que corra cada hora. Integrar con SendGrid o SES. Template HTML simple con link a la tarea.',
     'done', 'medium',
     'c3d4e5f6-a7b8-9012-cdef-123456789012', 'd4e5f6a7-b8c9-0123-defa-234567890123',
     NOW() - INTERVAL '2 days', 8),

    ('99999999-9999-9999-9999-999999999999',
     'Refactorizar el manejo de errores del backend',
     'Actualmente los handlers retornan errores inconsistentes. Algunos mandan JSON, otros texto plano. Crear un error handler centralizado con tipos de error (ValidationError, NotFoundError, AuthError) y respuestas consistentes. Incluir request ID para debugging.',
     'todo', 'medium',
     'a1b2c3d4-e5f6-7890-abcd-ef1234567890', NULL,
     NOW() + INTERVAL '12 days', 10),

    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
     'Implementar soft delete para tareas',
     'En vez de borrar tareas permanentemente, agregar un campo deleted_at y filtrar en los queries. Esto permite recuperar tareas borradas accidentalmente y mantener historial. Agregar endpoint para recuperar y para listar borradas (solo admin).',
     'todo', 'low',
     'b2c3d4e5-f6a7-8901-bcde-f12345678901', 'c3d4e5f6-a7b8-9012-cdef-123456789012',
     NOW() + INTERVAL '20 days', 6);

-- Edit History
INSERT INTO edit_history (task_id, user_id, field_name, old_value, new_value) VALUES
    ('11111111-1111-1111-1111-111111111111', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'status', 'todo', 'in_progress'),
    ('11111111-1111-1111-1111-111111111111', 'b2c3d4e5-f6a7-8901-bcde-f12345678901', 'priority', 'medium', 'high'),
    ('22222222-2222-2222-2222-222222222222', 'b2c3d4e5-f6a7-8901-bcde-f12345678901', 'priority', 'high', 'urgent'),
    ('44444444-4444-4444-4444-444444444444', 'c3d4e5f6-a7b8-9012-cdef-123456789012', 'status', 'in_progress', 'review'),
    ('66666666-6666-6666-6666-666666666666', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'status', 'todo', 'in_progress'),
    ('88888888-8888-8888-8888-888888888888', 'd4e5f6a7-b8c9-0123-defa-234567890123', 'status', 'in_progress', 'done');

-- Tags
INSERT INTO tags (id, name, color) VALUES
    ('6d3a3010-2534-42ab-904d-a87071451593', 'backend', '#3B82F6'),
    ('74a3ffcc-079d-463f-a6c9-8066de1d1eba', 'frontend', '#8B5CF6'),
    ('3e743db2-6000-4838-88e7-21d7493dbf94', 'bug', '#EF4444'),
    ('36ebf726-bb4b-4bfe-b83e-f236c274c52a', 'feature', '#10B981'),
    ('dda10acf-64c1-48cd-8b83-3deb4b1f2060', 'devops', '#F59E0B'),
    ('a1f79253-6078-4c1c-b8e2-3551d20d1467', 'security', '#EC4899'),
    ('5f91d74c-f563-41a3-b0fc-b65160b0a9ef', 'performance', '#06B6D4'),
    ('83fe6449-0ab8-48a1-b2e7-59c6760b4dfe', 'documentation', '#6B7280');

-- Task Tags (some tasks already have manual tags)
INSERT INTO task_tags (task_id, tag_id, assigned_by) VALUES
    ('11111111-1111-1111-1111-111111111111', '6d3a3010-2534-42ab-904d-a87071451593', 'manual'),
    ('11111111-1111-1111-1111-111111111111', '74a3ffcc-079d-463f-a6c9-8066de1d1eba', 'manual'),
    ('22222222-2222-2222-2222-222222222222', '3e743db2-6000-4838-88e7-21d7493dbf94', 'manual'),
    ('22222222-2222-2222-2222-222222222222', '36ebf726-bb4b-4bfe-b83e-f236c274c52a', 'manual'),
    ('33333333-3333-3333-3333-333333333333', 'dda10acf-64c1-48cd-8b83-3deb4b1f2060', 'manual'),
    ('44444444-4444-4444-4444-444444444444', 'a1f79253-6078-4c1c-b8e2-3551d20d1467', 'manual'),
    ('66666666-6666-6666-6666-666666666666', '5f91d74c-f563-41a3-b0fc-b65160b0a9ef', 'manual'),
    ('66666666-6666-6666-6666-666666666666', '83fe6449-0ab8-48a1-b2e7-59c6760b4dfe', 'manual');
