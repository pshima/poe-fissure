package poeapi

import (
	"context"
	"net/url"

	"github.com/peteshima/poe-fissure/internal/schema"
)

// realmSegment returns the optional path segment for a realm. PoE1 PC uses no
// segment; PoE2 uses "poe2"; console realms use "xbox"/"sony".
func realmSegment(realm string) string {
	switch realm {
	case "", "pc":
		return ""
	default:
		return "/" + realm
	}
}

// ListCharacters returns the account's characters for the given realm.
// Requires the account:characters scope.
func (c *Client) ListCharacters(ctx context.Context, realm string) ([]schema.Character, error) {
	var resp schema.CharacterListResponse
	if err := c.getJSON(ctx, "/character"+realmSegment(realm), &resp); err != nil {
		return nil, err
	}
	return resp.Characters, nil
}

// GetCharacter returns a single character (equipment, skills, jewels, passives)
// by name for the given realm. Requires the account:characters scope.
func (c *Client) GetCharacter(ctx context.Context, realm, name string) (*schema.Character, error) {
	var resp schema.CharacterResponse
	path := "/character" + realmSegment(realm) + "/" + url.PathEscape(name)
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Character, nil
}
