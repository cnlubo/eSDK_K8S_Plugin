package fibrechannel

import (
	"eSDK_K8S_Plugin/connector"
	"eSDK_K8S_Plugin/dev"
	"eSDK_K8S_Plugin/utils/log"
	"errors"
	"fmt"
	"time"
)

type FibreChannel struct{}

func init() {
	connector.RegisterConnector(connector.FCDriver, &FibreChannel{})
}

func (fc *FibreChannel) ConnectVolume(conn map[string]interface{}) (string, error) {
	log.Infof("FC Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		msg := "there is no Lun WWN in connect info"
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	scanHost()
	var device string
	var findDeviceMap map[string]string

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second * 3)
		device, _ = connector.GetDevice(findDeviceMap, tgtLunWWN)
		if device != "" {
			break
		}

		log.Warningf("Device of WWN %s wasn't found yet, will wait and check again", tgtLunWWN)
	}

	if device == "" {
		msg := fmt.Sprintf("Cannot detect device %s", tgtLunWWN)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return fmt.Sprintf("/dev/%s", device), nil
}

func (fc *FibreChannel) DisConnectVolume(tgtLunWWN string) error {
	log.Infof("Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return dev.DeleteDev(tgtLunWWN)
}
