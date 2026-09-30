// Author and committer identity checks shared by the edit panels.

// Mirrors validateIdentity in git/rewrite.go: these values would produce a
// malformed commit header that `git fsck` and many servers reject on push.
const IDENTITY_FORBIDDEN = /[<>\r\n]/

export interface IdentityErrors {
  name: string | null
  email: string | null
}

export const NO_IDENTITY_ERRORS: IdentityErrors = { name: null, email: null }

export function identityErrors(name: string, email: string, role = 'Author'): IdentityErrors {
  let nameError: string | null = null
  if (!name.trim()) {
    nameError = `${role} name cannot be empty.`
  } else if (IDENTITY_FORBIDDEN.test(name)) {
    nameError = `${role} name cannot contain < or >.`
  }
  const emailError = IDENTITY_FORBIDDEN.test(email) ? `${role} email cannot contain < or >.` : null
  return { name: nameError, email: emailError }
}

// What an edit does with the committer name and email, as sent in
// EditRequest.committer / BulkEditRequest.committer: keep them, copy the
// (new) author, or set them to the given values.
export type CommitterMode = 'keep' | 'author' | 'set'
