# 数据库备份与恢复指南

## 备份策略

- 脚本位置：`scripts/backup.sh`
- 备份目录：默认 `/opt/lab-teaching/backups`，可用 `BACKUP_DIR` 环境变量覆盖
- 保留策略：最近 30 天（脚本自动清理）
- 定时备份：`./scripts/backup.sh auto` 注册 crontab，每天凌晨 02:00 执行，日志写入备份目录 `backup.log`

## 常用操作

```bash
# 立即备份（需要数据库密码）
DB_PASSWORD='***' ./scripts/backup.sh backup

# 列出全部备份
./scripts/backup.sh list

# 恢复到最新备份（会要求输入 YES 二次确认）
DB_PASSWORD='***' ./scripts/backup.sh restore

# 恢复到指定文件
DB_PASSWORD='***' ./scripts/backup.sh restore /path/to/lab_teaching_xxx.sql.gz
```

> 本机部署无 root 权限时，加 `BACKUP_DIR=/home/ubuntu/lab-teaching/backups` 前缀。

## 恢复流程（生产事故应急）

1. **停止写入**：停止后端服务，避免恢复期间新数据写入
   ```bash
   systemctl stop lab-teaching   # 或 kill 对应进程
   ```
2. **执行恢复**：运行 `restore`，确认提示后输入 `YES`
3. **验证数据**：
   ```bash
   PGPASSWORD='***' psql -U postgres -d lab_teaching -c 'select count(*) from t_user;'
   ```
4. **重启服务**并抽查页面功能

## 验证记录

- 2026-08-07 实际执行备份验证通过：`lab_teaching_20260807_235202.sql.gz`（20K）
