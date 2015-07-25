package tcpp

import (
	"errors"
	"crypto/rand"
	"github.com/hsheth2/logs"
	"github.com/hsheth2/notifiers"
	"sync"
	"network/ipv4p"
)

func (c *TCB) UpdateState(newState uint) {
	logs.Info.Println("The New State is", newState)
	c.state = newState
	go SendUpdate(c.stateUpdate)
	if c.serverParent != nil {
		go SendUpdate(c.serverParent.connQueueUpdate)
	}
}

func (c *TCB) UpdateLastAck(newAck uint32) error {
	logs.Info.Println("New ack number:", newAck)
	c.recentAckNum = newAck
	go notifiers.SendNotifierBroadcast(c.recentAckUpdate, c.recentAckNum)
	return nil
}

func SendUpdate(update *sync.Cond) {
	update.L.Lock()
	update.Broadcast()
	update.L.Unlock()
}


type TCP_Packet struct {
	header   *TCP_Header
	payload  []byte
	rip, lip string
}

func (p *TCP_Packet) Marshal_TCP_Packet() ([]byte, error) {
	head, err := p.header.Marshal_TCP_Header(p.rip, p.lip)
	packet := append(head, p.payload...)
	return packet, err
}

func (p *TCP_Packet) getPayloadSize() uint32 {
	if len(p.payload) == 0 {
		return 1
	}
	return uint32(len(p.payload))
}

type TCP_Header struct {
	srcport, dstport uint16
	seq, ack         uint32
	// will do data offset automatically
	flags  uint8
	window uint16
	// checksum will be automatic
	urg     uint16
	options []byte
}

func (h *TCP_Header) Marshal_TCP_Header(dstIP, srcIP string) ([]byte, error) {
	// pad options with 0's
	for len(h.options)%4 != 0 {
		h.options = append(h.options, 0)
	}

	headerLen := uint16(TCP_BASIC_HEADER_SZ + len(h.options)) // size of header in 32 bit (4 byte) chunks

	header := append([]byte{
		(byte)(h.srcport >> 8), (byte)(h.srcport), // Source port in byte slice
		(byte)(h.dstport >> 8), (byte)(h.dstport), // Destination port in byte slice
		(byte)(h.seq >> 24), (byte)(h.seq >> 16), (byte)(h.seq >> 8), (byte)(h.seq), // seq
		(byte)(h.ack >> 24), (byte)(h.ack >> 16), (byte)(h.ack >> 8), (byte)(h.ack), // ack
		(byte)(
		(headerLen / 4) << 4, // data offset.
		// bits 5-7 inclusive are reserved, always 0
		// bit 8 is flag 0(NS flag), set to 0 here because only SYN
		),
		byte(h.flags),
		byte(h.window >> 8), byte(h.window), // window
		0, 0, // checksum (0 for now, set later)
		byte(h.urg >> 8), byte(h.urg), // URG pointer, only matters where URG flag is set
	}, h.options...)

	// insert the checksum
	cksum := ipv4p.CalcTransportChecksum(header, srcIP, dstIP, headerLen, ipv4p.TCP_PROTO)
	header[16] = byte(cksum >> 8)
	header[17] = byte(cksum)

	return header, nil
}

func Extract_TCP_Packet(d []byte, rip, lip string) (*TCP_Packet, error) {
	// TODO: test this function fully

	// header length
	headerLen := uint16((d[12] >> 4) * 4)
	if headerLen < TCP_BASIC_HEADER_SZ {
		return nil, errors.New("Bad TCP header size: Less than 20.")
	}

	// checksum verification
	if !ipv4p.VerifyTransportChecksum(d[:headerLen], rip, lip, headerLen, ipv4p.TCP_PROTO) {
		return nil, errors.New("Bad TCP header checksum")
	}

	// create the header
	h := &TCP_Header{
		srcport: uint16(d[0])<<8 | uint16(d[1]),
		dstport: uint16(d[2])<<8 | uint16(d[3]),
		seq:     uint32(d[4])<<24 | uint32(d[5])<<16 | uint32(d[6])<<8 | uint32(d[7]),
		ack:     uint32(d[8])<<24 | uint32(d[9])<<16 | uint32(d[10])<<8 | uint32(d[11]),
		flags:   uint8(d[13]),
		window:  uint16(d[14])<<8 | uint16(d[15]),
		urg:     uint16(d[18])<<8 | uint16(d[19]),
		options: d[TCP_BASIC_HEADER_SZ:headerLen],
	}
	return &TCP_Packet{header: h, payload: d[headerLen:], rip: rip, lip: lip}, nil
}

func genRandSeqNum() (uint32, error) {
	x := make([]byte, 4) // four bytes
	_, err := rand.Read(x)
	if err != nil {
		return 0, errors.New("genRandSeqNum gave error:" + err.Error())
	}
	return uint32(x[0])<<24 | uint32(x[1])<<16 | uint32(x[2])<<8 | uint32(x[3]), nil
}

func min(a, b uint64) uint64 {
	if a > b {
		return b
	}
	return a
}