package ipv4

import (
	"errors"
	"network/ethernet"

	"network/ipv4/ipv4tps"

	"github.com/hsheth2/logs"
)

type IP_Read_Manager struct {
	incoming chan *ethernet.Ethernet_Header
	buffers  map[uint8](map[*ipv4tps.IPaddress](chan []byte))
}

var GlobalIPReadManager = func() *IP_Read_Manager {
	irm, err := NewIP_Read_Manager(ethernet.GlobalNetworkReader)
	if err != nil {
		logs.Error.Fatal(err)
	}
	return irm
}()

func NewIP_Read_Manager(in *ethernet.Network_Reader) (*IP_Read_Manager, error) {
	input, err := in.Bind(ethernet.ETHERTYPE_IP)
	if err != nil {
		return nil, err
	}

	irm := &IP_Read_Manager{
		incoming: input,
		buffers:  make(map[uint8](map[*ipv4tps.IPaddress](chan []byte))),
	}

	go irm.readAll()

	return irm, nil
}

func (nr *IP_Read_Manager) readAll() {
	for {
		eth_packet := <-nr.incoming
		buf := eth_packet.Packet

		if len(buf) <= IP_HEADER_LEN {
			logs.Info.Println("Dropping IP Packet for bogus length <=", IP_HEADER_LEN)
			continue
		}

		protocol := uint8(buf[9])
		rip := &ipv4tps.IPaddress{buf[12:16]}

		//fmt.Println(ln)
		//fmt.Println(protocol, ip)
		/*if ln == 47 {
			fmt.Println(buf)
		}*/
		//fmt.Println(protocol, ip)
		if protoBuf, foundProto := nr.buffers[protocol]; foundProto {
			//fmt.Println("Dealing with packet")
			var output chan []byte = nil
			if c, foundIP := protoBuf[rip]; foundIP {
				//fmt.Println("Found exact")
				output = c
			} else if c, foundAll := protoBuf[&ipv4tps.IPaddress{IP: ipv4tps.IP_ALL}]; foundAll {
				//fmt.Println("Found global")
				output = c
			} else {
				logs.Warn.Println("output buf doesn't exist, rip:", rip)
				continue
			}
			select {
			case output <- buf:
			default:
				logs.Warn.Println("Dropping incoming IPv4 packet: no space in buffer")
			}
		}
	}
}

func (irm *IP_Read_Manager) Bind(ip *ipv4tps.IPaddress, protocol uint8) (chan []byte, error) {
	// create the protocol buffer if it doesn't exist already
	_, protoOk := irm.buffers[protocol]
	if !protoOk {
		irm.buffers[protocol] = make(map[*ipv4tps.IPaddress](chan []byte))
		//Trace.Println("Bound to", protocol)
	}

	// add the IP binding, if possible
	if _, IP_exists := irm.buffers[protocol][ip]; !IP_exists {
		// doesn't exist in map already
		irm.buffers[protocol][ip] = make(chan []byte, 1)

		ret, _ := irm.buffers[protocol][ip]
		return ret, nil
	}
	return nil, errors.New("IP already bound to.")
}

func (irm *IP_Read_Manager) Unbind(ip *ipv4tps.IPaddress, protocol uint8) error {
	ipBuf, protoOk := irm.buffers[protocol]
	if !protoOk {
		return errors.New("IP not bound, cannot unbind")
	}

	if _, ok := ipBuf[ip]; ok {
		delete(ipBuf, ip)
		return nil
	}
	return errors.New("Not bound, can't unbind.")
}
