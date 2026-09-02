package api

// CredentialCreateRequest asks the bootstrap administrator to mint a scoped API credential.
type CredentialCreateRequest struct {
	Scope     string `json:"scope"`
	Access    string `json:"access"`
	Namespace string `json:"namespace,omitempty"`
}

// CredentialCreateResponse returns the newly minted bearer token exactly once.
type CredentialCreateResponse struct {
	Token string `json:"token"`
}
