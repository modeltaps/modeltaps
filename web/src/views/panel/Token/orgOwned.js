// An organization API key the organization owns (ORG-16): its created_by is the account that pays
// for it (the organization), not a member. Personal keys have created_by 0.
export function isOrgOwnedToken(token) {
  return Boolean(token) && token.created_by > 0 && token.created_by === token.user_id;
}

// created_by filter value that lists the keys the organization owns.
export const ORG_OWNED_CREATOR_FILTER = '-1';
