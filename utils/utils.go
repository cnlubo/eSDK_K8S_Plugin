package utils

import (
	"eSDK_K8S_Plugin/utils/log"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DoradoV6Version = "V600R003C00"
)

type VolumeMetrics struct {
	Available *resource.Quantity
	Capacity *resource.Quantity
	InodesUsed *resource.Quantity
	Inodes *resource.Quantity
	InodesFree *resource.Quantity
	Used *resource.Quantity
}

func PathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func ExecShellCmd(format string, args ...interface{}) (string, error) {
	cmd := fmt.Sprintf(format, args...)
	log.Infof("Gonna run shell cmd \"%s\".", cmd)

	shCmd := exec.Command("/bin/sh", "-c", cmd)
	output, err := shCmd.CombinedOutput()
	if err != nil {
		log.Warningf("Run shell cmd \"%s\" error: %s.", cmd, output)
		return string(output), err
	}

	log.Infof("Shell cmd \"%s\" result:\n%s", cmd, output)
	return string(output), nil
}

func GetLunName(name string) string {
	if len(name) <= 31 {
		return name
	}

	return name[:31]
}

func GetSnapshotName(name string) string {
	if len(name) <= 31 {
		return name
	}

	return name[:31]
}

func GetFusionStorageLunName(name string) string {
	if len(name) <= 95 {
		return name
	}
	return name[:95]
}

func GetFusionStorageSnapshotName(name string) string {
	if len(name) <= 95 {
		return name
	}
	return name[:95]
}

func GetFileSystemName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func GetFSSnapshotName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func GetSharePath(name string) string {
	return "/" + strings.Replace(name, "-", "_", -1) + "/"
}

func GetFSSharePath(name string) string {
	return "/" + strings.Replace(name, "-", "_", -1) + "/"
}

func GetHostName() (string, error) {
	hostname, err := ExecShellCmd("hostname | xargs echo -n")
	if err != nil {
		return "", err
	}

	return hostname, nil
}

func GetPathTail(device string) string {
	strs := strings.Split(device, "/")
	if len(strs) > 0 {
		return strs[len(strs)-1]
	}
	return ""
}

func GetBackendAndVolume(volumeId string) (string, string) {
	var backend, volume string

	splits := strings.SplitN(volumeId, "-", 2)
	if len(splits) == 2 {
		backend, volume = splits[0], splits[1]
	} else {
		backend, volume = "", splits[0]
	}

	log.Infof("Backend %s, volume %s", backend, volume)
	return backend, volume
}

func SplitVolumeId(volumeId string) (string, string) {
	splits := strings.SplitN(volumeId, ".", 2)
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return splits[0], ""
}

func SplitSnapshotId(snapshotId string) (string, string, string) {
	splits := strings.SplitN(snapshotId, ".", 3)
	if len(splits) == 3 {
		return splits[0], splits[1], splits[2]
	}
	return splits[0], "", ""
}

func MergeMap(args ...map[string]interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})
	for _, arg := range args {
		for k, v := range arg {
			newMap[k] = v
		}
	}

	return newMap
}

func WaitUntil(f func() (bool, error), timeout time.Duration, interval time.Duration) error {
	done := make(chan error)

	go func() {
		timeout := time.After(timeout)

		for {
			condition, err := f()
			if err != nil {
				done <- err
				return
			}

			if condition {
				done <- nil
				return
			}

			select {
			case <-timeout:
				done <- fmt.Errorf("Wait timeout")
				return
			default:
				time.Sleep(interval)
			}
		}
	}()

	select {
	case err := <-done:
		return err
	}
}

func RandomInt(n int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(n)
}

func CopyMap(srcMap interface{}) map[string]interface{} {
	copied := make(map[string]interface{})

	if m, ok := srcMap.(map[string]string); ok {
		for k, v := range m {
			copied[k] = v
		}
	} else if m, ok := srcMap.(map[string]interface{}); ok {
		for k, v := range m {
			copied[k] = v
		}
	}

	return copied
}

func StrToBool(str string) bool {
	b, err := strconv.ParseBool(str)
	if err != nil {
		log.Warningf("Parse bool string %s error, return false")
		return false
	}

	return b
}

func ReflectCall(obj interface{}, method string, args ...interface{}) []reflect.Value {
	in := make([]reflect.Value, len(args))
	for i, v := range args {
		in[i] = reflect.ValueOf(v)
	}

	if v := reflect.ValueOf(obj).MethodByName(method); v.IsValid() {
		return v.Call(in)
	}

	return nil
}

func IsDoradoV6(SystemInfo map[string]interface{}) bool {
	versionInfo := SystemInfo["PRODUCTVERSION"].(string)
	return versionInfo >= DoradoV6Version
}

func IsSupportFeature(features map[string]int, feature string) bool {
	var support bool

	status, exist := features[feature]
	if exist {
		support = status == 1 || status == 2
	}

	return support
}

func TransVolumeCapacity(size int64, unit int64) int64 {
	newSize := RoundUpSize(size, unit)
	return newSize
}

func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	roundedUp := volumeSizeBytes / allocationUnitBytes
	if volumeSizeBytes%allocationUnitBytes > 0 {
		roundedUp++
	}
	return roundedUp
}

func GetAlua(alua map[string]interface{}, host string) map[string]interface{} {
	if alua == nil {
		return nil
	}

	for k, v := range alua {
		if k == "*" {
			continue
		}

		match, err := regexp.MatchString(k, host)
		if err != nil {
			log.Errorf("Regexp match error: %v", err)
		} else if match {
			return v.(map[string]interface{})
		}
	}

	return alua["*"].(map[string]interface{})
}

func fsInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(path, statfs)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	capacity := int64(statfs.Blocks) * int64(statfs.Bsize)
	available := int64(statfs.Bavail) * int64(statfs.Bsize)
	used := (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)

	inodes := int64(statfs.Files)
	inodesFree := int64(statfs.Ffree)
	inodesUsed := inodes - inodesFree
	return inodes, inodesFree, inodesUsed, available, capacity, used, nil
}

func GetVolumeMetrics(path string) (*VolumeMetrics, error) {
	volumeMetrics := &VolumeMetrics{}

	inodes, inodesFree, inodesUsed, available, capacity, usage, err := fsInfo(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get FsInfo, error %v", err)
	}
	volumeMetrics.Inodes = resource.NewQuantity(inodes, resource.BinarySI)
	volumeMetrics.InodesFree = resource.NewQuantity(inodesFree, resource.BinarySI)
	volumeMetrics.InodesUsed = resource.NewQuantity(inodesUsed, resource.BinarySI)
	volumeMetrics.Available = resource.NewQuantity(available, resource.BinarySI)
	volumeMetrics.Capacity = resource.NewQuantity(capacity, resource.BinarySI)
	volumeMetrics.Used = resource.NewQuantity(usage, resource.BinarySI)

	return volumeMetrics, nil
}

func GetLunUniqueId(protocol string, lun map[string]interface{}) (string, error){
	if protocol == "roce" || protocol == "fc-nvme" {
		tgtLunGuid, exist := lun["NGUID"].(string)
		if !exist {
			msg := fmt.Sprintf("The Lun info %s does not contain key NGUID", lun)
			log.Errorln(msg)
			return "", errors.New(msg)
		}
		return tgtLunGuid, nil
	} else {
		return lun["WWN"].(string), nil
	}
}

func GetAccessModeType(accessMode csi.VolumeCapability_AccessMode_Mode) string {
	switch accessMode {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
		return "ReadWrite"
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
		return "ReadOnly"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		return "ReadOnly"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
		return "ReadWrite"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		return "ReadWrite"
	default:
		return ""
	}
}
