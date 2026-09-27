package remote

import "github.com/buildset/buildset/pkg/safeurl"

// URLs are the identity service's pages. They are configuration rather than calls so rendering a
// page never stalls on a round trip, and they stay relative because the gateway puts both services
// on one origin, which is what keeps the session cookie and the same-origin checks working.
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
