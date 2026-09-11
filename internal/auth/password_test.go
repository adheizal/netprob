package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("correct password was rejected")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("wrong password was accepted")
	}
	if VerifyPassword("password", "invalid") {
		t.Fatal("malformed hash was accepted")
	}
}
