package api

import (
	"os"
	"os/exec"

	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) TriggerBackup(c *gin.Context) {
	cmd := exec.Command("/bin/bash", "/home/ubuntu/lab-teaching/scripts/backup.sh", "backup")
	cmd.Dir = "/home/ubuntu/lab-teaching"
	cmd.Env = append(os.Environ(),
		"BACKUP_DIR=/home/ubuntu/lab-teaching/backups",
		"PGPASSWORD="+h.Config.Database.Password,
		"DB_HOST="+h.Config.Database.Host,
		"DB_PORT="+h.Config.Database.Port,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		response.ServerError(c, "备份失败: "+err.Error()+"\n"+string(out))
		return
	}
	response.Success(c, gin.H{"output": string(out)})
}