-- 机房教学管理系统 - 数据库初始化脚本
-- 说明: 表结构由 GORM AutoMigrate 自动创建, 此脚本仅做库级初始化
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 时区设置
SET TIMEZONE = 'Asia/Shanghai';

-- 提示: 首次启动 gateway-api 时 AutoMigrate 会自动建表, 包括
-- t_user / t_room / t_device / t_agent / t_exam / t_exam_question /
-- t_campus / t_edu_sync_config / t_edu_sync_log / t_access_card_event /
-- t_access_rule / t_upgrade_task / t_upgrade_node 等
