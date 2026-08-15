package modbus

import (
	"bytes"
	"io"
	"net"
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func newTestServer(points ...*config.Point) *ModbusTCPServer {
	s := NewTCPServer(0, 1, "ABCD")
	s.SetStore(library.NewStore(points))
	return s
}

func TestReadHoldingRegistersFromNonZeroAddress(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeAI, Value: 12.5,
		FunctionCode: 3, RegisterAddress: 100,
	})

	got := s.handleRequest(3, []byte{0, 100, 0, 2})
	want := []byte{3, 4, 0x41, 0x48, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	reads, _, _ := s.Stats()
	if reads != 1 {
		t.Fatalf("read count = %d, want 1", reads)
	}
}

func TestWriteMultipleRegistersFloat32(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeAO,
		FunctionCode: 3, RegisterAddress: 100,
	})

	got := s.handleRequest(16, []byte{0, 100, 0, 2, 4, 0x41, 0x48, 0, 0})
	want := []byte{16, 0, 100, 0, 2}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	point, _ := s.store.Get(1)
	if point.Value != 12.5 {
		t.Fatalf("value = %v, want 12.5", point.Value)
	}
}

func TestMalformedMultiWriteRequestsReturnException(t *testing.T) {
	tests := []struct {
		name string
		fc   uint8
		data []byte
		want []byte
	}{
		{
			name: "coils byte count shorter than quantity",
			fc:   15,
			data: []byte{0, 100, 0, 9, 1, 0},
			want: []byte{0x8f, 3},
		},
		{
			name: "register byte count shorter than quantity",
			fc:   16,
			data: []byte{0, 100, 0, 2, 2, 0x41, 0x48},
			want: []byte{0x90, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer()
			got := s.handleRequest(tt.fc, tt.data)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("response = %x, want %x", got, tt.want)
			}
		})
	}
}

func TestSingleRegisterWriteRejects32BitPoint(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeAO,
		FunctionCode: 3, RegisterAddress: 100,
	})

	got := s.handleRequest(6, []byte{0, 100, 0x41, 0x48})
	want := []byte{0x86, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
}

func TestWriteSingleCoilRejectsInvalidEncoding(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeDO,
		FunctionCode: 1, RegisterAddress: 20,
	})

	got := s.handleRequest(5, []byte{0, 20, 0x12, 0x34})
	want := []byte{0x85, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
}

func TestFloat32ByteOrderRoundTrip(t *testing.T) {
	for _, order := range []string{"ABCD", "CDAB", "BADC", "DCBA"} {
		t.Run(order, func(t *testing.T) {
			got := RegistersToFloat32(Float32ToRegisters(12.5, order), order)
			if got != 12.5 {
				t.Fatalf("round-trip value = %v, want 12.5", got)
			}
		})
	}
}

func TestWriteMultipleCoilsUsesFC15Mapping(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeDO,
		FunctionCode: 15, RegisterAddress: 20,
	})

	got := s.handleRequest(15, []byte{0, 20, 0, 1, 1, 1})
	want := []byte{15, 0, 20, 0, 1}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	point, _ := s.store.Get(1)
	if !point.BoolValue {
		t.Fatal("FC15 mapped point was not updated")
	}
}

func TestWriteMultipleRegistersDoesNotPartiallyApplyInvalidRequest(t *testing.T) {
	s := newTestServer(
		&config.Point{IOA: 1, PointType: config.TypeAO, Value: 1, FunctionCode: 3, RegisterAddress: 100},
		&config.Point{IOA: 2, PointType: config.TypeAO, Value: 2, FunctionCode: 3, RegisterAddress: 102},
	)

	got := s.handleRequest(16, []byte{0, 100, 0, 3, 6, 0x41, 0x48, 0, 0, 0x42, 0x20})
	want := []byte{0x90, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	first, _ := s.store.Get(1)
	second, _ := s.store.Get(2)
	if first.Value != 1 || second.Value != 2 {
		t.Fatalf("partial update occurred: first=%v second=%v", first.Value, second.Value)
	}
}

func TestConnectionEchoesClientUnitID(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeAI, Value: 12.5,
		FunctionCode: 4, RegisterAddress: 100,
	})
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		s.handleConnection(serverConn)
		close(done)
	}()

	request := []byte{0, 1, 0, 0, 0, 6, 0xff, 4, 0, 100, 0, 2}
	if _, err := clientConn.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 13)
	if _, err := io.ReadFull(clientConn, response); err != nil {
		t.Fatal(err)
	}
	if response[6] != 0xff {
		t.Fatalf("response unit ID = %d, want 255", response[6])
	}
	clientConn.Close()
	<-done
}

func TestWriteMultipleRegistersUsesFC6Mapping(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeAO,
		FunctionCode: 6, RegisterAddress: 31,
	})

	got := s.handleRequest(16, []byte{0, 31, 0, 2, 4, 0x41, 0x48, 0, 0})
	want := []byte{16, 0, 31, 0, 2}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	point, _ := s.store.Get(1)
	if point.Value != 12.5 {
		t.Fatalf("value = %v, want 12.5", point.Value)
	}

	got = s.handleRequest(3, []byte{0, 31, 0, 2})
	want = []byte{3, 4, 0x41, 0x48, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("FC3 response for FC6 mapping = %x, want %x", got, want)
	}
}

func TestWriteMultipleCoilsUsesFC5Mapping(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 1, PointType: config.TypeDO,
		FunctionCode: 5, RegisterAddress: 20,
	})

	got := s.handleRequest(15, []byte{0, 20, 0, 1, 1, 1})
	want := []byte{15, 0, 20, 0, 1}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	point, _ := s.store.Get(1)
	if !point.BoolValue {
		t.Fatal("FC5 mapped point was not updated by FC15 request")
	}
}

func TestWriteSingleCoilUsesFC15MappingAndSyncsValue(t *testing.T) {
	s := newTestServer(&config.Point{
		IOA: 10, PointType: config.TypeDO,
		FunctionCode: 15, RegisterAddress: 21,
	})

	got := s.handleRequest(5, []byte{0, 21, 0xff, 0})
	want := []byte{5, 0, 21, 0xff, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	point, _ := s.store.Get(10)
	if !point.BoolValue || point.Value != 1 {
		t.Fatalf("expected BoolValue=true and Value=1, got BoolValue=%v Value=%v", point.BoolValue, point.Value)
	}

	got = s.handleRequest(5, []byte{0, 21, 0, 0})
	want = []byte{5, 0, 21, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %x, want %x", got, want)
	}
	if point.BoolValue || point.Value != 0 {
		t.Fatalf("expected BoolValue=false and Value=0, got BoolValue=%v Value=%v", point.BoolValue, point.Value)
	}
}
