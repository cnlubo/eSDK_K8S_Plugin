#BUILD_VERSION   := $(shell cat build_version)
#BUILD_DATE      := $(shell date '+%Y-%m-%d %H:%M:%S')
#COMMIT_SHA1     := $(shell git rev-parse --short HEAD)

VERSION_PKG     :=
DEST_DIR        := bin
APP             := huawei-csi
APP1            := passwdEncrypt

huaweicsi:
	gox -osarch="darwin/amd64 linux/amd64" \
        -output='${DEST_DIR}/${APP}_{{.OS}}_{{.Arch}}'  ./csi

passEn:
	gox -osarch="darwin/amd64 linux/amd64" \
        -output='${DEST_DIR}/${APP1}_{{.OS}}_{{.Arch}}'  ./tools/passwdEncrypt
clean:
	rm -rf ${DEST_DIR}
all: clean huaweicsi passEn

.EXPORT_ALL_VARIABLES:

GO111MODULE = on
