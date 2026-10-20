//go:build windows

// Package etw consumes a real-time Event Tracing for Windows session.
//
// It is deliberately minimal: just enough to read the kernel network
// provider, which is the only way Windows reports how many bytes each
// process sends and receives. Only the byte counts and the process id are
// read from each event; no packet content is ever looked at.
package etw

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32           = windows.NewLazySystemDLL("advapi32.dll")
	procStartTraceW    = advapi32.NewProc("StartTraceW")
	procControlTraceW  = advapi32.NewProc("ControlTraceW")
	procEnableTraceEx2 = advapi32.NewProc("EnableTraceEx2")
	procOpenTraceW     = advapi32.NewProc("OpenTraceW")
	procProcessTrace   = advapi32.NewProc("ProcessTrace")
	procCloseTrace     = advapi32.NewProc("CloseTrace")
)

const (
	wnodeFlagTracedGuid            = 0x00020000
	eventTraceRealTimeMode         = 0x00000100
	processTraceModeRealTime       = 0x00000100
	processTraceModeEventRecord    = 0x10000000
	eventControlCodeEnableProvider = 1
	traceLevelVerbose              = 5

	controlCodeStop = 1

	invalidProcessTraceHandle = ^uintptr(0)

	// Offsets inside EVENT_TRACE_PROPERTIES, which is 120 bytes on x64.
	propsSize = 120
)

// wnodeHeader is the first member of EVENT_TRACE_PROPERTIES (48 bytes).
type wnodeHeader struct {
	BufferSize    uint32
	ProviderId    uint32
	HistoricalCtx uint64
	TimeStamp     int64
	Guid          windows.GUID
	ClientContext uint32
	Flags         uint32
}

type eventTraceProperties struct {
	Wnode               wnodeHeader
	BufferSize          uint32
	MinimumBuffers      uint32
	MaximumBuffers      uint32
	MaximumFileSize     uint32
	LogFileMode         uint32
	FlushTimer          uint32
	EnableFlags         uint32
	AgeLimit            int32
	NumberOfBuffers     uint32
	FreeBuffers         uint32
	EventsLost          uint32
	BuffersWritten      uint32
	LogBuffersLost      uint32
	RealTimeBuffersLost uint32
	LoggerThreadId      uintptr
	LogFileNameOffset   uint32
	LoggerNameOffset    uint32
}

// eventTraceLogfile mirrors EVENT_TRACE_LOGFILEW. The two embedded structs
// are kept as byte arrays: their sizes are multiples of 8, so every field
// after them keeps the alignment the C layout has.
type eventTraceLogfile struct {
	LogFileName         *uint16
	LoggerName          *uint16
	CurrentTime         int64
	BuffersRead         uint32
	ProcessTraceMode    uint32
	CurrentEvent        [88]byte
	LogfileHeader       [280]byte
	BufferCallback      uintptr
	BufferSize          uint32
	Filled              uint32
	EventsLost          uint32
	_                   uint32
	EventRecordCallback uintptr
	IsKernelTrace       uint32
	_                   uint32
	Context             uintptr
}

type eventDescriptor struct {
	Id      uint16
	Version uint8
	Channel uint8
	Level   uint8
	Opcode  uint8
	Task    uint16
	Keyword uint64
}

type eventHeader struct {
	Size            uint16
	HeaderType      uint16
	Flags           uint16
	EventProperty   uint16
	ThreadId        uint32
	ProcessId       uint32
	TimeStamp       int64
	ProviderId      windows.GUID
	EventDescriptor eventDescriptor
	_               [8]byte
	ActivityId      windows.GUID
}

// EventRecord is one ETW event as delivered to the callback.
type eventRecord struct {
	EventHeader       eventHeader
	BufferContext     uint32
	ExtendedDataCount uint16
	UserDataLength    uint16
	ExtendedData      unsafe.Pointer
	UserData          unsafe.Pointer
	UserContext       unsafe.Pointer
}

// Event is what a Handler receives: the event id and its raw payload.
type Event struct {
	ID   uint16
	Data []byte
}

// Handler is called for every event, on the trace's own thread. It must not
// block: anything slow belongs in another goroutine.
type Handler func(Event)

// Session is a real-time ETW session.
type Session struct {
	name     string
	provider windows.GUID
	handler  Handler

	control uintptr // session handle, for ControlTrace
	trace   uintptr // consumer handle, for ProcessTrace
	cb      uintptr
}

// NewSession describes a session. Nothing is started until Start.
func NewSession(name string, provider windows.GUID, h Handler) *Session {
	return &Session{name: name, provider: provider, handler: h}
}

// properties builds an EVENT_TRACE_PROPERTIES with room for the name.
func (s *Session) properties() ([]byte, *eventTraceProperties) {
	name := windows.StringToUTF16(s.name)
	buf := make([]byte, propsSize+len(name)*2)
	p := (*eventTraceProperties)(unsafe.Pointer(&buf[0]))
	p.Wnode.BufferSize = uint32(len(buf))
	p.Wnode.Flags = wnodeFlagTracedGuid
	p.Wnode.ClientContext = 1 // QPC timestamps
	p.LogFileMode = eventTraceRealTimeMode
	p.BufferSize = 64 // KB
	p.MinimumBuffers = 4
	p.MaximumBuffers = 16
	p.FlushTimer = 1
	p.LoggerNameOffset = propsSize
	copy(buf[propsSize:], unsafe.Slice((*byte)(unsafe.Pointer(&name[0])), len(name)*2))
	return buf, p
}

// stopStale stops a session of the same name left behind by a previous run,
// which would otherwise make StartTrace fail with ERROR_ALREADY_EXISTS.
func (s *Session) stopStale() {
	buf, _ := s.properties()
	name, _ := windows.UTF16PtrFromString(s.name)
	procControlTraceW.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&buf[0])), controlCodeStop)
}

// Start opens the session and enables the provider. It needs administrator
// rights: starting a trace session is a privileged operation.
func (s *Session) Start() error {
	if unsafe.Sizeof(eventTraceProperties{}) != propsSize {
		return fmt.Errorf("EVENT_TRACE_PROPERTIES is %d bytes, expected %d",
			unsafe.Sizeof(eventTraceProperties{}), propsSize)
	}
	s.stopStale()

	buf, _ := s.properties()
	name, err := windows.UTF16PtrFromString(s.name)
	if err != nil {
		return err
	}
	r, _, _ := procStartTraceW.Call(uintptr(unsafe.Pointer(&s.control)),
		uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		if windows.Errno(r) == windows.ERROR_ACCESS_DENIED {
			return fmt.Errorf("StartTrace: access denied — this must run as administrator")
		}
		return fmt.Errorf("StartTrace: %w", windows.Errno(r))
	}

	r, _, _ = procEnableTraceEx2.Call(s.control, uintptr(unsafe.Pointer(&s.provider)),
		eventControlCodeEnableProvider, traceLevelVerbose,
		0xFFFFFFFFFFFFFFFF, 0, 0, 0)
	if r != 0 {
		s.Stop()
		return fmt.Errorf("EnableTraceEx2: %w", windows.Errno(r))
	}
	return nil
}

// Process consumes events until Stop is called. It blocks, and pins itself
// to one OS thread because ProcessTrace calls back on the calling thread.
func (s *Session) Process() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	s.cb = windows.NewCallback(func(rec *eventRecord) uintptr {
		if rec.UserData != nil && rec.UserDataLength > 0 && s.handler != nil {
			s.handler(Event{
				ID:   rec.EventHeader.EventDescriptor.Id,
				Data: unsafe.Slice((*byte)(rec.UserData), int(rec.UserDataLength)),
			})
		}
		return 0
	})

	name, err := windows.UTF16PtrFromString(s.name)
	if err != nil {
		return err
	}
	lf := eventTraceLogfile{
		LoggerName:          name,
		ProcessTraceMode:    processTraceModeRealTime | processTraceModeEventRecord,
		EventRecordCallback: s.cb,
	}
	h, _, _ := procOpenTraceW.Call(uintptr(unsafe.Pointer(&lf)))
	if h == invalidProcessTraceHandle {
		return fmt.Errorf("OpenTrace: %w", windows.GetLastError())
	}
	s.trace = h

	r, _, _ := procProcessTrace.Call(uintptr(unsafe.Pointer(&s.trace)), 1, 0, 0)
	procCloseTrace.Call(s.trace)
	s.trace = 0
	// ProcessTrace returns ERROR_CANCELLED after CloseTrace from Stop.
	if r != 0 && windows.Errno(r) != windows.ERROR_CANCELLED {
		return fmt.Errorf("ProcessTrace: %w", windows.Errno(r))
	}
	return nil
}

// Stop ends the session, which makes Process return.
func (s *Session) Stop() {
	if s.trace != 0 {
		procCloseTrace.Call(s.trace)
	}
	if s.control != 0 {
		buf, _ := s.properties()
		procControlTraceW.Call(s.control, 0, uintptr(unsafe.Pointer(&buf[0])), controlCodeStop)
		s.control = 0
	}
}
