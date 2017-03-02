default:
	go build

ws:
	GOOS=windows GOARCH=386 go build

arm:
	GOOS=android GOARCH=arm \
		 CC=$(GOPATH)/pkg/gomobile/android-ndk-r10e/arm/bin/arm-linux-androideabi-gcc \
		 CXX=$(GOPATH)/pkg/gomobile/android-ndk-r10e/arm/bin/arm-linux-androideabi-g++ \
		 CGO_ENABLED=1 go build
	# adb -s 88a2ac4f push -p ./proxy-server /data/
