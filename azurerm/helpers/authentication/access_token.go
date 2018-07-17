package authentication

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure/cli"
)

type AccessToken struct {
	ClientID     string
	AccessToken  *adal.Token
	IsCloudShell bool
}

func findValidAccessTokenForTenant(tokens []cli.Token, tenantId string) (*AccessToken, error) {
	mostRecentAccessToken := AccessToken{}
	foundToken := false
	for _, accessToken := range tokens {
		token, err := accessToken.ToADALToken()
		if err != nil {
			return nil, fmt.Errorf("[DEBUG] Error converting access token to token: %+v", err)
		}

		expirationDate, err := cli.ParseExpirationDate(accessToken.ExpiresOn)
		if err != nil {
			return nil, fmt.Errorf("Error parsing expiration date: %q", accessToken.ExpiresOn)
		}

		if expirationDate.UTC().Before(time.Now().UTC()) && accessToken.RefreshToken == "" {
			log.Printf("[DEBUG] Token %q has expired and it doens't have a refresh token", token.AccessToken)
			continue
		}

		if mostRecentAccessToken.AccessToken != nil &&
			expirationDate.UTC().After(mostRecentAccessToken.AccessToken.Expires()) {
			log.Printf("[DEBUG] Token %q has later expiration date", token.AccessToken)
		}

		if !strings.Contains(accessToken.Resource, "management") {
			log.Printf("[DEBUG] Resource %q isn't a management domain", accessToken.Resource)
			continue
		}

		if !strings.HasSuffix(accessToken.Authority, tenantId) {
			log.Printf("[DEBUG] Resource %q isn't for the correct Tenant", accessToken.Resource)
			continue
		}

		mostRecentAccessToken = AccessToken{
			ClientID:     accessToken.ClientID,
			AccessToken:  &token,
			IsCloudShell: accessToken.RefreshToken == "",
		}
		foundToken = true
	}
	if foundToken {
		return &mostRecentAccessToken, nil
	}
	return nil, fmt.Errorf("No Access Token was found for the Tenant ID %q", tenantId)
}
