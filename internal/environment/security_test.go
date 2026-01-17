package environment

import (
	"testing"
)

func TestSecurityInterceptor_ValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		allowPath string
		wantErr   bool
	}{
		{
			name:      "valid path in working dir",
			path:      "test/file.txt",
			allowPath: "/tmp/test",
			wantErr:   false,
		},
		{
			name:      "path traversal attack",
			path:      "../../../etc/passwd",
			allowPath: "/tmp/test",
			wantErr:   true,
		},
		{
			name:      "absolute path outside allowed",
			path:      "/etc/passwd",
			allowPath: "/tmp/test",
			wantErr:   true,
		},
		{
			name:      "valid absolute path in allowed",
			path:      "/tmp/test/file.txt",
			allowPath: "/tmp/test",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si := NewSecurityInterceptor(tt.allowPath, SecurityLevelStandard)
			si.allowedDirs = []string{tt.allowPath}

			err := si.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurityInterceptor_ValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		securityLevel SecurityLevel
		cmd         string
		args        []string
		wantErr     bool
	}{
		{
			name:          "safe command in strict mode",
			securityLevel: SecurityLevelStrict,
			cmd:           "ls",
			args:          []string{"-la"},
			wantErr:       false,
		},
		{
			name:          "unsafe command in strict mode",
			securityLevel: SecurityLevelStrict,
			cmd:           "rm",
			args:          []string{"-rf", "test"},
			wantErr:       true,
		},
		{
			name:          "command injection attempt",
			securityLevel: SecurityLevelStandard,
			cmd:           "ls",
			args:          []string{"; rm -rf /"},
			wantErr:       true,
		},
		{
			name:          "pipe injection attempt",
			securityLevel: SecurityLevelStandard,
			cmd:           "cat",
			args:          []string{"file.txt | rm file.txt"},
			wantErr:       true,
		},
		{
			name:          "dangerous command blacklist",
			securityLevel: SecurityLevelPermissive,
			cmd:           "rm",
			args:          []string{"-rf", "/"},
			wantErr:       true,
		},
		{
			name:          "fork bomb detection",
			securityLevel: SecurityLevelPermissive,
			cmd:           ":(){ :|:& };:",
			args:          []string{},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si := NewSecurityInterceptor("/tmp", tt.securityLevel)
			err := si.ValidateCommand(tt.cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurityInterceptor_ValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		filename string
		wantErr bool
	}{
		{
			name:     "valid filename",
			filename: "test.txt",
			wantErr:  false,
		},
		{
			name:     "filename with special characters",
			filename: "test<>.txt",
			wantErr:  true,
		},
		{
			name:     "path traversal in filename",
			filename: "../test.txt",
			wantErr:  true,
		},
		{
			name:     "windows reserved name",
			filename: "CON.txt",
			wantErr:  true,
		},
		{
			name:     "null byte in filename",
			filename: "test\x00.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si := NewSecurityInterceptor("/tmp", SecurityLevelStandard)
			err := si.ValidateFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilename() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurityInterceptor_SanitizeEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		securityLevel SecurityLevel
		env           map[string]string
		wantMinSize   int
	}{
		{
			name:          "strict mode removes dangerous vars",
			securityLevel: SecurityLevelStrict,
			env: map[string]string{
				"PATH":   "/usr/bin",
				"HOME":   "/home/user",
				"GOOS":   "linux",
				"IF":     "dangerous",
			},
			wantMinSize: 2, // HOME and GOOS should remain
		},
		{
			name:          "standard mode keeps some vars",
			securityLevel: SecurityLevelStandard,
			env: map[string]string{
				"PATH":   "/usr/bin",
				"HOME":   "/home/user",
				"CUSTOM": "value",
			},
			wantMinSize: 2, // HOME and CUSTOM should remain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si := NewSecurityInterceptor("/tmp", tt.securityLevel)
			result := si.SanitizeEnvironment(tt.env)

			if len(result) < tt.wantMinSize {
				t.Errorf("SanitizeEnvironment() returned %d vars, want at least %d", len(result), tt.wantMinSize)
			}

			// Check that dangerous variables are removed
			if _, exists := result["IFS"]; exists {
				t.Error("SanitizeEnvironment() should remove IFS variable")
			}
		})
	}
}

func TestSecurityInterceptor_AddAllowedCommand(t *testing.T) {
	si := NewSecurityInterceptor("/tmp", SecurityLevelStrict)

	// Initially, rm should not be allowed
	err := si.ValidateCommand("rm", []string{"test"})
	if err == nil {
		t.Error("Expected error for 'rm' command in strict mode")
	}

	// Add rm to allowed commands
	si.AddAllowedCommand("rm")

	// Now it should be allowed
	err = si.ValidateCommand("rm", []string{"test"})
	if err != nil {
		t.Errorf("Expected no error after adding 'rm' to whitelist: %v", err)
	}
}

func TestSecurityInterceptor_SetSecurityLevel(t *testing.T) {
	si := NewSecurityInterceptor("/tmp", SecurityLevelStrict)

	// In strict mode, rm should not be allowed
	err := si.ValidateCommand("rm", []string{"test"})
	if err == nil {
		t.Error("Expected error for 'rm' command in strict mode")
	}

	// Change to permissive mode
	si.SetSecurityLevel(SecurityLevelPermissive)

	// Now rm should be allowed (as long as it's not dangerous)
	err = si.ValidateCommand("rm", []string{"test"})
	if err != nil {
		t.Errorf("Expected no error in permissive mode: %v", err)
	}
}
