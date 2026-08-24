//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRegistrationEmailSuffixWhitelist(t *testing.T) {
	got, err := NormalizeRegistrationEmailSuffixWhitelist([]string{"example.com", "@EXAMPLE.COM", " @foo.bar ", "*.EDU.CN"})
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar", "*.edu.cn"}, got)
}

func TestNormalizeRegistrationEmailSuffixWhitelist_Invalid(t *testing.T) {
	for _, item := range []string{"@invalid_domain", "*.", "*", "*.@", "*.foo"} {
		t.Run(item, func(t *testing.T) {
			_, err := NormalizeRegistrationEmailSuffixWhitelist([]string{item})
			require.Error(t, err)
		})
	}
}

func TestParseRegistrationEmailSuffixWhitelist(t *testing.T) {
	got := ParseRegistrationEmailSuffixWhitelist(`["example.com","@foo.bar","*.EDU.CN","@invalid_domain","*.foo"]`)
	require.Equal(t, []string{"@example.com", "@foo.bar", "*.edu.cn"}, got)
}

func TestIsRegistrationEmailSuffixBlocked(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		blacklist []string
		want      bool
	}{
		{name: "exact domain", email: "user@mail.tm", blacklist: []string{"@mail.tm"}, want: true},
		{name: "wildcard subdomain", email: "user@mx.tempmail.example", blacklist: []string{"*.tempmail.example"}, want: true},
		{name: "normal provider remains allowed", email: "user@gmail.com", blacklist: []string{"@mail.tm", "*.tempmail.example"}, want: false},
		{name: "empty blacklist", email: "user@mail.tm", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsRegistrationEmailSuffixBlocked(tt.email, tt.blacklist))
		})
	}
}

func TestIsRegistrationEmailMultiLevelDomainBlocked(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "normal gmail", email: "user@gmail.com", want: false},
		{name: "normal qq", email: "user@qq.com", want: false},
		{name: "two label custom domain", email: "user@example.co", want: false},
		{name: "normal uk public suffix", email: "user@example.co.uk", want: false},
		{name: "normal china public suffix", email: "user@school.edu.cn", want: false},
		{name: "three label generated domain", email: "user@a.example.com", want: true},
		{name: "deep generated domain", email: "user@mx.mail.example.com", want: true},
		{name: "invalid email", email: "not-an-email", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsRegistrationEmailMultiLevelDomainBlocked(tt.email))
		})
	}
}

func TestIsRegistrationEmailSuffixAllowed(t *testing.T) {
	require.True(t, IsRegistrationEmailSuffixAllowed("user@example.com", []string{"@example.com"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("user@sub.example.com", []string{"@example.com"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@qq.com", []string{"@qq.com"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("user@sub.qq.com", []string{"@qq.com"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("student@cs.edu.cn", []string{"*.edu.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("student@edu.cn", []string{"*.edu.cn"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("student@foo.cn", []string{"*.edu.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@a.com", []string{"@a.com", "*.b.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@school.b.cn", []string{"@a.com", "*.b.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@b.cn", []string{"@a.com", "*.b.cn"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("user@c.cn", []string{"@a.com", "*.b.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@any.com", []string{}))
}
