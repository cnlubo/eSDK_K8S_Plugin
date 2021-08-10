package driver


type Driver struct {
	name    string
	version string
}



//func (d *Driver) ControllerGetVolume(ctx context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
//	panic("implement me")
//}

func NewDriver(name, version string) *Driver {
	return &Driver{
		name:    name,
		version: version,
	}
}
