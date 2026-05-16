package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "supersecret123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty hash")
	}

	if hash == password {
		t.Error("HashPassword returned plain text - password was not hashed")
	}

	if len(hash) < 4 || (hash[:4] != "$2a$" && hash[:4] != "$2b$") {
		t.Errorf("HashPassword returned unexpected format: %s", hash[:10])
	}

}

func TestHashPasswordSalting(t *testing.T) {
	password := "supersecret123"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("first HashPassword failed: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword produced identical hashes - salting is not working")
	}

}

func TestCheckPassword(t *testing.T) {
	correctPassword := "correctPassword"

	hash, err := HashPassword(correctPassword)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	tests := []struct {
		name      string
		password  string
		hash      string
		wantError bool
	}{
		{
			name:      "correct password matches hash",
			password:  correctPassword,
			hash:      hash,
			wantError: false,
		},
		{
			name:      "wrong password",
			password:  "wrongpassword",
			hash:      hash,
			wantError: true,
		},
		{
			name:      "empty password",
			password:  "",
			hash:      hash,
			wantError: true,
		},
		{
			name:      "similar but wrong password",
			password:  "correctPassword ",
			hash:      hash,
			wantError: true,
		},
		{
			name:      "case sensitive lowecase password",
			password:  "correctpassword",
			hash:      hash,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.password, tt.hash)
			gotError := err != nil
			if gotError != tt.wantError {
				t.Errorf("CheckPassword(%q) error=%v, wantError=%v",
					tt.password, err, tt.wantError)
			}
		})
	}

}

func TestHashAndVerifyRoundTrip(t *testing.T) {
	passwords := []string{
		"simple",
		"C0mpl3x!P@ssw0rd",
		"with spaces inside",
		"123456",
		"unicode: café",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("HashPassword failed: %v", err)
			}

			if err := CheckPassword(password, hash); err != nil {
				t.Errorf("CheckPassword failed after HashPassword: %v", err)
			}
		})
	}
}
