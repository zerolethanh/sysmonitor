package main

// ProcessInfo holds basic information about a running process.
type ProcessInfo struct {
	PID  int32
	Name string
	CPU  float64
	Mem  float32
}

// ConnInfo holds information about a network connection.
type ConnInfo struct {
	PID         int32
	ProcessName string
	LocalAddr   string
	RemoteAddr  string
	Status      string
}
