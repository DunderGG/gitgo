// Author identity checks shared by the edit panels.

// Mirrors validateIdentity in git/rewrite.go: these values would produce a
// malformed commit header that `git fsck` and many servers reject on push.
const IDENTITY_FORBIDDEN = /[<>\r\n]/

export function identityErrors(name: string, email: string): { name: string | null; email: string | null } {
  let nameError: string | null = null
  if (!name.trim()) {
    nameError = 'Author name cannot be empty.'
  } else if (IDENTITY_FORBIDDEN.test(name)) {
    nameError = 'Author name cannot contain < or >.'
  }
  const emailError = IDENTITY_FORBIDDEN.test(email) ? 'Author email cannot contain < or >.' : null
  return { name: nameError, email: emailError }
}
