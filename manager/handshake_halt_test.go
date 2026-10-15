package manager

import "testing"

func TestConnectManagerSkipsWhenHalted(t *testing.T) {
	m := &NodeHandshake{}
	depStatus := &ServiceStatus{
		managerCon: &ManagerConnection{
			halt:   true,
			status: NewConnectionStatus(STATUS_MANAGER_UNSET),
		},
	}

	err := m.connectManager("context", depStatus)
	if err != nil {
		t.Fatalf("connectManager: %v", err)
	}
	if depStatus.managerCon.status.Get() != STATUS_MANAGER_UNSET {
		t.Fatalf("status = %s, want %s", depStatus.managerCon.status.Get(), STATUS_MANAGER_UNSET)
	}
	if depStatus.managerCon.lockGoroutine {
		t.Fatal("halted connectManager locked the manager connection")
	}
}

func TestHandshakeDepStopsBeforeEvaluateStatusWhenHalted(t *testing.T) {
	m := &NodeHandshake{}
	depStatus := &ServiceStatus{
		managerCon: &ManagerConnection{
			halt:   true,
			status: NewConnectionStatus(STATUS_MANAGER_SECURED),
		},
	}

	m.handshakeDep("context", depStatus)

	if depStatus.managerCon.status.Get() != STATUS_MANAGER_SECURED {
		t.Fatalf("status = %s, want %s", depStatus.managerCon.status.Get(), STATUS_MANAGER_SECURED)
	}
}
