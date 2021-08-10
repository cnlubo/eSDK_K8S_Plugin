package fibrechannel

import (
	"eSDK_K8S_Plugin/utils"
	"eSDK_K8S_Plugin/utils/log"
)

func scanHost() {
	output, err := utils.ExecShellCmd("for host in $(ls /sys/class/fc_host/); " +
		"do echo \"- - -\" > /sys/class/scsi_host/${host}/scan; done")
	if err != nil {
		log.Warningf("rescan fc_host error: %s", output)
	}
}
