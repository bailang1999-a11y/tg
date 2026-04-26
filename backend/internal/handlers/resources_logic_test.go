package handlers

import "testing"

func TestValidateTerminalCheckSelection(t *testing.T) {
	tests := []struct {
		name        string
		groupID     string
		terminalID  string
		wantErr     string
		wantGroup   bool
		wantTerminal bool
	}{
		{name: "none", groupID: "", terminalID: "", wantGroup: false, wantTerminal: false},
		{name: "group only", groupID: "00000000-0000-0000-0000-000000000001", terminalID: "", wantGroup: true, wantTerminal: false},
		{name: "terminal only", groupID: "", terminalID: "00000000-0000-0000-0000-000000000002", wantGroup: false, wantTerminal: true},
		{name: "both", groupID: "00000000-0000-0000-0000-000000000001", terminalID: "00000000-0000-0000-0000-000000000002", wantErr: "终端组和终端不能同时选择"},
		{name: "bad group", groupID: "bad", terminalID: "", wantErr: "终端组 ID 无效"},
		{name: "bad terminal", groupID: "", terminalID: "bad", wantErr: "终端 ID 无效"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupID, terminalID, err := validateTerminalCheckSelection(tt.groupID, tt.terminalID)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("validateTerminalCheckSelection() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTerminalCheckSelection() unexpected error = %v", err)
			}
			if (groupID != nil) != tt.wantGroup {
				t.Fatalf("group presence = %v, want %v", groupID != nil, tt.wantGroup)
			}
			if (terminalID != nil) != tt.wantTerminal {
				t.Fatalf("terminal presence = %v, want %v", terminalID != nil, tt.wantTerminal)
			}
		})
	}
}
