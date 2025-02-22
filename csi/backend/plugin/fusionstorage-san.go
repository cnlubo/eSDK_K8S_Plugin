package plugin

import (
	"eSDK_K8S_Plugin/dev"
	"eSDK_K8S_Plugin/proto"
	"eSDK_K8S_Plugin/storage/fusionstorage/attacher"
	"eSDK_K8S_Plugin/storage/fusionstorage/volume"
	"eSDK_K8S_Plugin/utils"
	"eSDK_K8S_Plugin/utils/log"
	"errors"
	"fmt"
	"net"
	"strings"
)

type FusionStorageSanPlugin struct {
	FusionStoragePlugin
	hosts    map[string]string
	protocol string
	portals  []string
	alua     map[string]interface{}
}

func init() {
	RegPlugin("fusionstorage-san", &FusionStorageSanPlugin{})
}

func (p *FusionStorageSanPlugin) NewPlugin() Plugin {
	return &FusionStorageSanPlugin{
		hosts: make(map[string]string),
	}
}

func (p *FusionStorageSanPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	scsi, scsiExist := parameters["SCSI"].(map[string]interface{})
	iscsi, iscsiExist := parameters["ISCSI"].([]interface{})
	if !scsiExist && !iscsiExist {
		return errors.New("SCSI or ISCSI must be provided for fusionstorage-san")
	} else if scsiExist && iscsiExist {
		return errors.New("Provide only one of SCSI and ISCSI for fusionstorage-san")
	} else if scsiExist {
		for k, v := range scsi {
			manageIP := v.(string)
			ip := net.ParseIP(manageIP)
			if ip == nil {
				return fmt.Errorf("Manage IP %s of host %s is invalid", manageIP, k)
			}

			p.hosts[k] = manageIP
		}

		p.protocol = "scsi"
	} else {
		portals, err := proto.VerifyIscsiPortals(iscsi)
		if err != nil {
			return err
		}

		p.portals = portals
		p.protocol = "iscsi"
		p.alua, _ = parameters["ALUA"].(map[string]interface{})
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) getParams(name string, parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":     name,
		"capacity": utils.RoundUpSize(parameters["size"].(int64), CAPACITY_UNIT),
	}

	paramKeys := []string{
		"storagepool",
		"cloneFrom",
		"sourceSnapshotName",
		"sourceVolumeName",
		"snapshotParentId",
		"qos",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key].(string); exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	return params, nil
}

func (p *FusionStorageSanPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params, err := p.getParams(name, parameters)
	if err != nil {
		return "", err
	}

	san := volume.NewSAN(p.cli)
	err = san.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *FusionStorageSanPlugin) DeleteVolume(name string) error {
	san := volume.NewSAN(p.cli)
	return san.Delete(name)
}

func (p *FusionStorageSanPlugin) ExpandVolume(name string, size int64) (bool, error) {
	san := volume.NewSAN(p.cli)
	newSize := utils.TransVolumeCapacity(size, CAPACITY_UNIT)
	isAttach, err := san.Expand(name, newSize)
	return isAttach, err
}

func (p *FusionStorageSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	_, err := localAttacher.ControllerDetach(name, parameters)
	if err != nil {
		log.Errorf("Detach volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	cli := p.cli.DuplicateClient()
	err := cli.Login()
	if err != nil {
		return err
	}

	defer cli.Logout()

	localAttacher := attacher.NewAttacher(cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	devPath, err := localAttacher.NodeStage(name, parameters)
	if err != nil {
		log.Errorf("Stage volume %s error: %v", name, err)
		return err
	}

	targetPath := parameters["targetPath"].(string)
	fsType := parameters["fsType"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err = dev.MountLunDev(devPath, targetPath, fsType, mountFlags)
	if err != nil {
		log.Errorf("Mount device %s to %s error: %v", devPath, targetPath, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount %s error: %v", targetPath, err)
		return err
	}

	cli := p.cli.DuplicateClient()
	err = cli.Login()
	if err != nil {
		return err
	}

	defer cli.Logout()

	localAttacher := attacher.NewAttacher(cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	err = localAttacher.NodeUnstage(name, parameters)
	if err != nil {
		log.Errorf("Unstage volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   true,
	}

	return capabilities, nil
}

func (p *FusionStorageSanPlugin) NodeExpandVolume(name, volumePath string) error {
	cli := p.cli.DuplicateClient()
	err := cli.Login()
	if err != nil {
		return err
	}
	defer cli.Logout()

	lun, err := cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get lun %s error: %v", name, err)
		return err
	}
	if lun == nil {
		msg := fmt.Sprintf("LUN %s to expand doesn't exist", name)
		log.Errorln(msg)
		return errors.New(msg)
	}

	wwn := lun["wwn"].(string)
	err = dev.BlockResize(wwn)
	if err != nil {
		log.Errorf("Lun %s resize error: %v", wwn, err)
		return err
	}

	err = dev.ResizeMountPath(volumePath)
	if err != nil {
		log.Errorf("MountPath %s resize error: %v", volumePath, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	snapshot, err := san.CreateSnapshot(lunName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (p *FusionStorageSanPlugin) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	err := san.DeleteSnapshot(snapshotName)
	if err != nil {
		return err
	}

	return nil
}
