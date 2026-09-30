import { GetGitIdentity } from '../../wailsjs/go/app/App'
import { DATE_BUTTON_CLASS } from './DateShiftButtons'
import { errorText } from '../errors'
import { useRepoStore } from '../store/repoStore'

interface UseMyIdentityButtonProps {
  disabled: boolean
  onIdentity: (name: string, email: string) => void
}

// Tiny button that fills the author fields with the identity Git uses for new
// commits (user.name / user.email), the usual fix for a commit made with the
// wrong identity.
export default function UseMyIdentityButton({ disabled, onIdentity }: UseMyIdentityButtonProps) {
  const setError = useRepoStore((s) => s.setError)

  async function handleClick() {
    try {
      const identity = await GetGitIdentity()
      if (!identity.name && !identity.email) {
        setError('Git has no user.name or user.email set. Set them with git config, or type the author in.')
        return
      }
      onIdentity(identity.name, identity.email)
    } catch (error) {
      setError(errorText(error))
    }
  }

  return (
    <button
      type="button"
      title="Fill in user.name and user.email from your Git config"
      disabled={disabled}
      onClick={handleClick}
      className={DATE_BUTTON_CLASS}
    >
      Use my identity
    </button>
  )
}
