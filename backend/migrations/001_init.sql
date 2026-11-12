-- 机房教学管理系统 数据库初始化 V1.0
-- PostgreSQL 16 / 达梦兼容（达梦: BIGSERIAL→IDENTITY, JSONB→CLOB, BOOLEAN→CHAR(1)）

-- ============ 系统域 ============
CREATE TABLE IF NOT EXISTS t_user (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(256) NOT NULL,
    real_name VARCHAR(64),
    role VARCHAR(32) DEFAULT 'teacher',   -- admin/sys_admin/sec_admin/auditor/teacher
    status VARCHAR(16) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_audit (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT,
    action VARCHAR(128),
    target VARCHAR(256),
    ip VARCHAR(64),
    detail TEXT,
    created_at TIMESTAMP DEFAULT now()
);

-- ============ 资产域 ============
CREATE TABLE IF NOT EXISTS t_room (
    id BIGSERIAL PRIMARY KEY,
    building VARCHAR(64),
    floor VARCHAR(16),
    name VARCHAR(128) NOT NULL UNIQUE,
    seat_count INT DEFAULT 0,
    net_type VARCHAR(16) DEFAULT '千兆',
    boot_mode VARCHAR(16) DEFAULT '混合',  -- 无盘/本地/混合
    status VARCHAR(16) DEFAULT '正常',
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_device (
    id BIGSERIAL PRIMARY KEY,
    asset_code VARCHAR(64) UNIQUE,
    category VARCHAR(32) DEFAULT '电脑',
    brand VARCHAR(64),
    model VARCHAR(64),
    cpu_arch VARCHAR(16) DEFAULT 'x86_64',  -- x86_64/arm64/loongarch/mips64el/sw_64
    os VARCHAR(64),
    mac VARCHAR(32),
    ip VARCHAR(64),
    status VARCHAR(16) DEFAULT '在用',
    purchase_date DATE,
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_seat (
    id BIGSERIAL PRIMARY KEY,
    room_id BIGINT REFERENCES t_room(id),
    seat_no VARCHAR(16),
    device_id BIGINT REFERENCES t_device(id),
    boot_mode VARCHAR(16) DEFAULT '无盘',
    status VARCHAR(16) DEFAULT '空闲',
    UNIQUE(room_id, seat_no)
);
CREATE INDEX IF NOT EXISTS idx_seat_room ON t_seat(room_id);

-- ============ 教学域 ============
CREATE TABLE IF NOT EXISTS t_course (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    credit DECIMAL(3,1),
    teacher_id BIGINT REFERENCES t_user(id),
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_clazz (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    department VARCHAR(128),
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_student (
    id BIGSERIAL PRIMARY KEY,
    sno VARCHAR(32) UNIQUE,
    name VARCHAR(64),
    clazz_id BIGINT REFERENCES t_clazz(id),
    card_no VARCHAR(64),      -- 一卡通号
    face_id VARCHAR(128),     -- 人脸特征引用
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_schedule (
    id BIGSERIAL PRIMARY KEY,
    course_id BIGINT REFERENCES t_course(id),
    clazz_id BIGINT REFERENCES t_clazz(id),
    room_id BIGINT REFERENCES t_room(id),
    teacher_id BIGINT REFERENCES t_user(id),
    weekday INT,
    period VARCHAR(16),
    week_range VARCHAR(32),
    status VARCHAR(16) DEFAULT '正常',
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_online_log (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT REFERENCES t_student(id),
    seat_id BIGINT REFERENCES t_seat(id),
    login_at TIMESTAMP DEFAULT now(),
    logout_at TIMESTAMP,
    auth_type VARCHAR(16) DEFAULT '账密',
    duration INT DEFAULT 0
);

-- ============ Agent 管控域 ============
CREATE TABLE IF NOT EXISTS t_agent_session (
    id BIGSERIAL PRIMARY KEY,
    seat_id BIGINT REFERENCES t_seat(id),
    device_id BIGINT REFERENCES t_device(id),
    agent_id VARCHAR(64) UNIQUE,
    version VARCHAR(32),
    ip VARCHAR(64),
    mac VARCHAR(64),
    os VARCHAR(64),
    arch VARCHAR(16),
    boot_mode VARCHAR(16),
    online BOOLEAN DEFAULT false,
    last_heartbeat TIMESTAMP,
    created_at TIMESTAMP DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_agent_online ON t_agent_session(online);

CREATE TABLE IF NOT EXISTS t_broadcast_task (
    id BIGSERIAL PRIMARY KEY,
    room_id BIGINT REFERENCES t_room(id),
    teacher_id BIGINT REFERENCES t_user(id),
    scope VARCHAR(32) DEFAULT '全班',
    targets JSONB,
    mcast_addr VARCHAR(32),
    status VARCHAR(16) DEFAULT '进行中',
    record_path VARCHAR(256),
    start_at TIMESTAMP DEFAULT now(),
    end_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS t_remote_ctrl (
    id BIGSERIAL PRIMARY KEY,
    seat_id BIGINT REFERENCES t_seat(id),
    operator_id BIGINT REFERENCES t_user(id),
    mode VARCHAR(16) DEFAULT '监看',
    record_path VARCHAR(256),
    start_at TIMESTAMP DEFAULT now(),
    end_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS t_file_transfer (
    id BIGSERIAL PRIMARY KEY,
    direction VARCHAR(8),     -- down/up
    room_id BIGINT REFERENCES t_room(id),
    operator_id BIGINT REFERENCES t_user(id),
    file_name VARCHAR(256),
    size BIGINT,
    sha256 VARCHAR(64),
    targets JSONB,
    status VARCHAR(16) DEFAULT '传输中',
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_disk_image (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128),
    os VARCHAR(64),
    arch VARCHAR(16),
    path VARCHAR(256),
    version VARCHAR(32),
    course_map JSONB,
    status VARCHAR(16) DEFAULT '可用',
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_policy (
    id BIGSERIAL PRIMARY KEY,
    room_id BIGINT REFERENCES t_room(id),
    usb VARCHAR(16) DEFAULT '允许',
    print VARCHAR(16) DEFAULT '允许',
    net_mode VARCHAR(16) DEFAULT '开放',
    url_list JSONB,
    time_range VARCHAR(64),
    created_at TIMESTAMP DEFAULT now()
);

-- ============ 考试域 ============
CREATE TABLE IF NOT EXISTS t_question (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(16),         -- single/multi/judge/fill/short/code/op
    content TEXT,
    options JSONB,
    answer TEXT,
    analysis TEXT,
    difficulty INT DEFAULT 3,
    score DECIMAL(5,1) DEFAULT 5,
    knowledge VARCHAR(128),
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_paper (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128),
    total_score DECIMAL(6,1),
    duration INT,
    pass_line DECIMAL(5,1),
    shuffle BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_paper_question (
    id BIGSERIAL PRIMARY KEY,
    paper_id BIGINT REFERENCES t_paper(id),
    question_id BIGINT REFERENCES t_question(id),
    score DECIMAL(5,1),
    seq INT
);

CREATE TABLE IF NOT EXISTS t_exam (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128),
    paper_id BIGINT REFERENCES t_paper(id),
    strategy VARCHAR(32) DEFAULT '随机组卷',
    status VARCHAR(16) DEFAULT '草稿',
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_exam_arrange (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT REFERENCES t_exam(id),
    room_id BIGINT REFERENCES t_room(id),
    session VARCHAR(32),
    start_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS t_exam_candidate (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT REFERENCES t_exam(id),
    student_id BIGINT REFERENCES t_student(id),
    verify_type VARCHAR(16),
    verify_status VARCHAR(16) DEFAULT '待核验',
    seat_id BIGINT REFERENCES t_seat(id)
);

CREATE TABLE IF NOT EXISTS t_answer (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT REFERENCES t_exam(id),
    student_id BIGINT REFERENCES t_student(id),
    question_id BIGINT REFERENCES t_question(id),
    content TEXT,
    score DECIMAL(5,1),
    cached BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_exam_event (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT REFERENCES t_exam(id),
    student_id BIGINT REFERENCES t_student(id),
    type VARCHAR(32),
    detail TEXT,
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS t_score (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT REFERENCES t_exam(id),
    student_id BIGINT REFERENCES t_student(id),
    auto_score DECIMAL(6,1),
    manual_score DECIMAL(6,1),
    total DECIMAL(6,1),
    review VARCHAR(16) DEFAULT '待审',
    created_at TIMESTAMP DEFAULT now()
);

-- ============ 种子数据 ============
-- 默认管理员 admin / ChangeMe@123 (bcrypt hash 由后端启动时自动创建，可用 LAB_ADMIN_INIT_PASSWORD 覆盖)

INSERT INTO t_room (building, floor, name, seat_count, net_type, boot_mode, status) VALUES
('教学楼A', '1F', '机房A101', 60, '千兆', '无盘', '正常'),
('教学楼A', '2F', '机房A201', 60, '千兆', '混合', '正常'),
('实训楼B', '1F', '机房B101', 80, '万兆', '无盘', '正常'),
('实训楼B', '2F', '机房B201', 50, '千兆', '本地', '正常')
ON CONFLICT (name) DO NOTHING;

INSERT INTO t_course (name, credit) VALUES
('计算机基础', 3.0),
('Python程序设计', 4.0),
('数据库原理', 3.5),
('操作系统', 4.0)
ON CONFLICT DO NOTHING;

INSERT INTO t_clazz (name, department) VALUES
('计科2301', '计算机学院'),
('计科2302', '计算机学院'),
('软工2301', '软件学院')
ON CONFLICT DO NOTHING;

INSERT INTO t_disk_image (name, os, arch, path, version, status) VALUES
('麒麟桌面-教学基础', '银河麒麟V10 SP1', 'x86_64', '/images/kylin-edu-base.qcow2', '1.0', '可用'),
('统信UOS-编程实训', '统信UOS 1070', 'arm64', '/images/uos-dev.qcow2', '1.0', '可用'),
('openEuler-服务器实训', 'openEuler 24.03', 'x86_64', '/images/openeuler-lab.qcow2', '1.0', '可用')
ON CONFLICT DO NOTHING;

INSERT INTO t_policy (room_id, usb, print, net_mode, time_range) VALUES
(NULL, '允许', '允许', '开放', '08:00-22:00'),
(1, '禁止', '禁止', '白名单', '考试时段')
ON CONFLICT DO NOTHING;
