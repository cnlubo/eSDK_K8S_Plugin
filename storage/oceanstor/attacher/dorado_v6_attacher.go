package attacher

import (
	"eSDK_K8S_Plugin/storage/oceanstor/client"
	"eSDK_K8S_Plugin/utils"
	"eSDK_K8S_Plugin/utils/log"
)

type DoradoV6Attacher struct {
	Attacher
}

const (
	ACCESS_MODE_BALANCED = "0"
)

func newDoradoV6Attacher(
	cli *client.Client,
	protocol, invoker string,
	portals []string,
	alua map[string]interface{}) AttacherPlugin {
	return &DoradoV6Attacher{
		Attacher: Attacher{
			cli:      cli,
			protocol: protocol,
			invoker:  invoker,
			portals:  portals,
			alua:     alua,
		},
	}
}

func (p *DoradoV6Attacher) needUpdateHost(host map[string]interface{}, hostAlua map[string]interface{}) bool {
	accessMode, ok := hostAlua["accessMode"]
	if !ok {
		return false
	}

	if accessMode != host["accessMode"] {
		return true
	} else if host["accessMode"] == ACCESS_MODE_BALANCED {
		return false
	}

	hyperMetroPathOptimized, ok := hostAlua["hyperMetroPathOptimized"]
	if ok && hyperMetroPathOptimized != host["hyperMetroPathOptimized"] {
		return true
	}

	return false
}

func (p *DoradoV6Attacher) ControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {
	host, err := p.getHost(parameters, true)
	if err != nil {
		log.Errorf("Get host ID error: %v", err)
		return "", err
	}

	hostID := host["ID"].(string)
	hostAlua := utils.GetAlua(p.alua, host["NAME"].(string))

	if hostAlua != nil && p.needUpdateHost(host, hostAlua) {
		err := p.cli.UpdateHost(hostID, hostAlua)
		if err != nil {
			log.Errorf("Update host %s error: %v", hostID, err)
			return "", err
		}
	}

	if p.protocol == "iscsi" {
		_, err = p.Attacher.attachISCSI(hostID)
	} else if p.protocol == "fc" || p.protocol == "fc-nvme" {
		_, err = p.Attacher.attachFC(hostID)
	} else if p.protocol == "roce" {
		_, err = p.Attacher.attachRoCE(hostID)
	}

	if err != nil {
		log.Errorf("Attach %s connection error: %v", p.protocol, err)
		return "", err
	}

	wwn, err := p.doMapping(hostID, lunName)
	if err != nil {
		log.Errorf("Mapping LUN %s to host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}

func (p *DoradoV6Attacher) NodeStage(lunName string, parameters map[string]interface{}) (string, error) {
	return connectVolume(p, lunName, p.protocol, parameters)
}
