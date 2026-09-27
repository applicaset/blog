package remote

import "github.com/buildset/buildset/pkg/safeurl"

// URLs are the identity service's pages. They are configuration, not calls, so rendering never
// waits on a round trip. They stay relative because the gateway serves both services on one
// origin, which the session cookie and the same-origin checks depend on.
type URLs struct {
	Setup    string
	Login    string
	Logout   string
	Password string
	NewUser  string
}

// DefaultURLs are the paths the identity service serves.
func DefaultURLs() URLs {
	return URLs{
		Setup:    "/setup",
		Login:    "/login",
		Logout:   "/logout",
		Password: "/password",
		NewUser:  "/users/new",
	}
}

// The redirect target goes through the same check the identity service applies, from the same
// package, so the guarantee cannot hold on one side of the gateway and not the other.
func (u URLs) LoginURL(next string) string    { return safeurl.WithNext(u.Login, next) }
func (u URLs) LogoutURL(next string) string   { return safeurl.WithNext(u.Logout, next) }
func (u URLs) PasswordURL(next string) string { return safeurl.WithNext(u.Password, next) }
func (u URLs) NewUserURL(next string) string  { return safeurl.WithNext(u.NewUser, next) }
func (u URLs) SetupURL(next string) string    { return safeurl.WithNext(u.Setup, next) }
