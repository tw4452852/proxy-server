package proxy_server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

type cmd int

const (
	getVmAddress cmd = iota
	getLocalAddress
	exit
)

type op struct {
	cmd              cmd
	localAddr        chan string
	vmConfig, vmData chan string
	err              chan error
}

type web struct {
	ln               net.Listener
	ops              chan *op
	vmConfig, vmData string
}

func NewWeb() *web {
	w := &web{
		ops: make(chan *op),
	}

	go w.loop()

	return w
}

func (w *web) Address() (addr string, err error) {
	op := &op{
		cmd:       getLocalAddress,
		localAddr: make(chan string, 1),
		err:       make(chan error, 1),
	}
	w.ops <- op
	return <-op.localAddr, <-op.err
}

func (w *web) Exit() error {
	op := &op{
		cmd: exit,
		err: make(chan error, 1),
	}
	w.ops <- op
	return <-op.err
}

func (w *web) GetVmAddress() (config, data string, err error) {
	op := &op{
		cmd:      getVmAddress,
		vmConfig: make(chan string, 1),
		vmData:   make(chan string, 1),
		err:      make(chan error, 1),
	}
	w.ops <- op
	return <-op.vmConfig, <-op.vmData, <-op.err
}

func (w *web) loop() {
	for op := range w.ops {
		w.handleOp(op)

		if op.cmd == exit {
			return
		}
	}
}

func (w *web) handleOp(op *op) {
	switch op.cmd {
	case getLocalAddress:
		if w.ln != nil {
			op.localAddr <- w.ln.Addr().String()
			op.err <- nil
			return
		}
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			log.Printf("[web]: %s\n", err)
			op.localAddr <- ""
			op.err <- err
			return
		}

		go func() {
			for {
				// Wait for a connection.
				conn, err := ln.Accept()
				if err != nil {
					log.Printf("[web]: %s\n", err)
					return
				}
				// Handle the connection in a new goroutine.
				// The loop then returns to accepting, so that
				// multiple connections may be served concurrently.
				go handleClientMsg(conn)
			}
		}()
		w.ln = ln

		op.localAddr <- w.ln.Addr().String()
		op.err <- nil
		return
	case getVmAddress:
		c, d, e := getVmAddresses()
		op.vmConfig <- c
		op.vmData <- d
		op.err <- e
		return
	case exit:
		if w.ln != nil {
			w.ln.Close()
		}
		return
	default:
		log.Printf("[web]: unknown cmd %#x\n", op.cmd)
		return
	}
}

func handleClientMsg(c net.Conn) {
	defer c.Close()
	for {
		tlv, err := ReadTLV(c)
		if err != nil {
			log.Printf("[web]: %s\n", err)
			return
		}
		Debug.Printf("[web]: get client msg, t[%#x], l[%d], v[%v]\n",
			tlv.T, tlv.L, tlv.V)
	}
}

func getVmAddresses() (vmConfig, vmData string, err error) {
	var (
		body []byte
	)
	const (
		IMEI = "123456789"
	)

	post := func(url string, body []byte) ([]byte, error) {
		resp, err := http.Post(url, "", bytes.NewReader(body))
		if err != nil {
			log.Printf("[web]: post [%s] failed: %s\n", url, err)
			return nil, err
		}

		body, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[web]: ReadAll failed: %s\n", err)
			return nil, err
		}
		Debug.Printf("[web]: post[%s] return : StatusCode[%d], body[%s]\n",
			url, resp.StatusCode, string(body))
		return body, nil
	}

	// get task system info
	body, err = post("http://www.mdatp.com:14280/AutoSvrMgr/GetTaskSysInfo.action", nil)
	if err != nil {
		return
	}

	type info struct {
		Level             string `json:"Level"`
		Id                string `json:"ID"`
		Url               string `json:"SysInterfaceUrlPath"`
		MaxEexecutionTime string `json:"MaxEexecutionTime"`
	}
	type sys struct {
		Code                 string `json:"Code"`
		ReportHeartbeatCycle string `json:"ReportHeartbeatCycle"`
		GetVmAddrCycle       string `json:"GetVmAddrCycle"`
		Infos                []info `json:"TaskSysInfoArray"`
	}

	var s sys
	err = json.Unmarshal(body, &s)
	if err != nil {
		log.Printf("[web]: %s\n", err)
		return
	}
	Debug.Printf("[web]: system info: %#v\n", s)

	// report device info
	type devinfo struct {
		IMEI                string `json:"IMEI"`
		NetworkOperatorName string `json:"NetworkOperatorName"`
		NetworkType         string `json:"NetworkType"`
		ProductID           string `json:"ProductID"`
		Product             string `json:"Product"`
		Device              string `json:"Device"`
		Board               string `json:"Board"`
		CPU_ABI             string `json:"CPU_ABI"`
		Manufacturer        string `json:"Manufacturer"`
		Brand               string `json:"Brand"`
		Model               string `json:"Model"`
		Bootloader          string `json:"Bootloader"`
		SystemVersion       string `json:"SystemVersion"`
		Latitude            string `json:"Latitude"`
		Longitude           string `json:"Longitude"`
		Altitude            string `json:"Altitude"`
		Accuracy            string `json:"Accuracy"`
		IP                  string `json:"IP"`
		GetSMSCodeStatus    string `json:"GetSMSCodeStatus"`
		MacAddress          string `json:"MacAddress"`
	}

	di := devinfo{
		IMEI:                IMEI,
		NetworkOperatorName: "123",
		NetworkType:         "123",
		ProductID:           "123",
		Product:             "123",
		Device:              "123",
		Board:               "123",
		CPU_ABI:             "123",
		Manufacturer:        "123",
		Brand:               "123",
		Model:               "123",
		Bootloader:          "123",
		SystemVersion:       "123",
		Latitude:            "123",
		Longitude:           "123",
		Altitude:            "123",
		Accuracy:            "123",
		IP:                  "1.1.1.1",
		GetSMSCodeStatus:    "1",
	}
	url := s.Infos[0].Url + "/ReportDevInfo.action"
	body, err = json.Marshal(di)
	if err != nil {
		log.Printf("[web]: %s\n", err)
		return
	}
	body, err = post(url, body)
	if err != nil {
		return
	}

	// get vm sys address
	url = s.Infos[0].Url + "/GetVmPlatAddr.action"
	body, err = json.Marshal(struct {
		IMEI string `json:"IMEI"`
	}{
		IMEI: IMEI,
	})
	if err != nil {
		log.Printf("[web]: %s\n", err)
		return
	}
	body, err = post(url, body)
	if err != nil {
		return
	}
	vmPlat := struct {
		VmPlatIP   string `json:"VmPlatIP"`
		VmPlatPort string `json:"VmPlatPort"`
	}{}
	err = json.Unmarshal(body, &vmPlat)
	if err != nil {
		log.Printf("[web]: %s\n", err)
		return
	}
	Debug.Printf("[web]: vm platform info: %#v\n", vmPlat)

	// get vm address
	url = "http://" + vmPlat.VmPlatIP + ":" + vmPlat.VmPlatPort + "/AutoTestPlatform/GetVmAddr.action"
	body, err = json.Marshal(struct {
		IMEI    string `json:"IMEI"`
		AppCode string `json:"AppCode"`
	}{
		IMEI:    IMEI,
		AppCode: "123",
	})
	body, err = post(url, body)
	if err != nil {
		return
	}
	vm := struct {
		VmIP       string `json:"VmIP"`
		VmCtrlPort string `json:"VmCtrlPort"`
		VmDataPort string `json:"VmDataPort"`
	}{}
	err = json.Unmarshal(body, &vm)
	if err != nil {
		log.Printf("[web]: %s\n", err)
		return
	}
	Debug.Printf("[web]: vm info: %#v\n", vm)

	return vm.VmIP + ":" + vm.VmCtrlPort, vm.VmIP + ":" + vm.VmDataPort, nil
}
