package token

type AnonPrincipal struct{}

func (p *AnonPrincipal) Token() string         { return "" }
func (p *AnonPrincipal) Name() string          { return "anonymous" }
func (p *AnonPrincipal) IsAuthenticated() bool { return false }

func NewAnonymousPrincipal() *AnonPrincipal {
	return &AnonPrincipal{}
}
