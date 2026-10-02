package git

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// BackupRefPrefix is the namespace that holds backups. A backup is one ref per
// branch, all sharing an ID:
//
//	refs/gitgo/backups/<id>/<branch>
//
// The refs keep the backed-up commits alive with no expiry (`git gc` never
// prunes a commit a ref points to), are not pushed by `git push`, even with
// --all, and do not show up in `git branch`.
const BackupRefPrefix = "refs/gitgo/backups/"

// backupTimeLayout formats a backup's creation time in its ID. Ref names cannot
// contain ':', and the fixed-width UTC form sorts chronologically.
const backupTimeLayout = "2006-01-02T15-04-05.000Z"

// reflogRestorePrefix starts the reflog message of a restore.
const reflogRestorePrefix = "gitgo: restore backup "

// maxRestoreListed caps the commits listed per branch in a RestorePlan.
const maxRestoreListed = 20

// BackupKind says why a backup was made.
type BackupKind string

const (
	// BackupManual is made by the user and kept until deleted.
	BackupManual BackupKind = "manual"
	// BackupAuto is made by GitGo before an edit or restore; only the newest
	// few per branch are kept (see PruneBackups).
	BackupAuto BackupKind = "auto"
)

// BackupBranch is one branch saved in a backup.
type BackupBranch struct {
	// Name is the short branch name, e.g. "feature/x".
	Name string
	// Hash is the saved commit. For a named backup the ref points at a tag
	// object holding the name, and Hash is the commit it peels to.
	Hash plumbing.Hash
}

// maxBackupNameLength is the longest backup name, in characters.
const maxBackupNameLength = 100

// backupTagName is the tag name recorded in a named backup's tag objects. It
// is never a ref under refs/tags/, so it shows up nowhere else.
const backupTagName = "gitgo-backup"

// Backup is a saved set of branch tips.
type Backup struct {
	// ID is the shared ref path segment, e.g. "2026-09-25T14-03-00.000Z-auto".
	ID      string
	Kind    BackupKind
	Created time.Time
	// Name is the user's name for the backup, or empty (see SetBackupName).
	Name string
	// Branches are sorted by name.
	Branches []BackupBranch
}

// Tip returns the commit the backup saved for branch.
func (backup Backup) Tip(branch string) (plumbing.Hash, bool) {
	for _, saved := range backup.Branches {
		if saved.Name == branch {
			return saved.Hash, true
		}
	}
	return plumbing.ZeroHash, false
}

// backupRefName returns the ref holding branch in the backup with the given ID.
func backupRefName(id, branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(BackupRefPrefix + id + "/" + branch)
}

// backupID builds an ID from a creation time and kind.
func backupID(created time.Time, kind BackupKind) string {
	return created.UTC().Format(backupTimeLayout) + "-" + string(kind)
}

// parseBackupID splits an ID into its creation time and kind.
func parseBackupID(id string) (time.Time, BackupKind, bool) {
	separator := strings.LastIndex(id, "-")
	if separator < 0 {
		return time.Time{}, "", false
	}
	kind := BackupKind(id[separator+1:])
	if kind != BackupManual && kind != BackupAuto {
		return time.Time{}, "", false
	}
	created, err := time.Parse(backupTimeLayout, id[:separator])
	if err != nil {
		return time.Time{}, "", false
	}
	return created, kind, true
}

// CreateBackup saves the current tip of state.Branch, and of each branch in
// others that exists, as a new backup. Nothing is copied: the backup is only a
// set of refs, so it is instant whatever the repository size.
func CreateBackup(state *RepoState, kind BackupKind, others []string) (Backup, error) {
	if kind != BackupManual && kind != BackupAuto {
		return Backup{}, fmt.Errorf("unknown backup kind %q", kind)
	}

	tip, err := branchTip(state)
	if err != nil {
		return Backup{}, err
	}
	saved := map[string]plumbing.Hash{state.Branch: tip.Hash()}
	for _, name := range others {
		if _, done := saved[name]; done {
			continue
		}
		ref, err := state.Repo.Reference(plumbing.NewBranchReferenceName(name), true)
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			continue
		}
		if err != nil {
			return Backup{}, fmt.Errorf("reading branch %s: %w", name, err)
		}
		saved[name] = ref.Hash()
	}

	existing, err := ListBackups(state.Repo)
	if err != nil {
		return Backup{}, err
	}
	taken := make(map[string]bool, len(existing))
	for _, backup := range existing {
		taken[backup.ID] = true
	}

	// IDs have millisecond precision; move a clashing one forward so every
	// backup keeps its own refs and the order of creation.
	created := time.Now().UTC().Truncate(time.Millisecond)
	for taken[backupID(created, kind)] {
		created = created.Add(time.Millisecond)
	}

	backup := Backup{ID: backupID(created, kind), Kind: kind, Created: created}
	for name, hash := range saved {
		backup.Branches = append(backup.Branches, BackupBranch{Name: name, Hash: hash})
	}
	sort.Slice(backup.Branches, func(i, j int) bool { return backup.Branches[i].Name < backup.Branches[j].Name })

	for i, branch := range backup.Branches {
		ref := plumbing.NewHashReference(backupRefName(backup.ID, branch.Name), branch.Hash)
		if err := state.Repo.Storer.SetReference(ref); err != nil {
			// Leave no half-written backup behind.
			for _, written := range backup.Branches[:i] {
				_ = state.Repo.Storer.RemoveReference(backupRefName(backup.ID, written.Name))
			}
			return Backup{}, fmt.Errorf("writing backup ref: %w", err)
		}
	}
	return backup, nil
}

// ListBackups returns every backup in the repository, newest first. Refs under
// BackupRefPrefix that GitGo did not write are ignored.
func ListBackups(repo *gogit.Repository) ([]Backup, error) {
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("listing refs: %w", err)
	}
	defer refs.Close()

	byID := make(map[string]*Backup)
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if ref.Type() != plumbing.HashReference || !strings.HasPrefix(name, BackupRefPrefix) {
			return nil
		}
		id, branch, found := strings.Cut(strings.TrimPrefix(name, BackupRefPrefix), "/")
		if !found || branch == "" {
			return nil
		}
		if _, _, valid := parseBackupID(id); !valid {
			return nil
		}
		// A named backup's refs point at tag objects carrying the name.
		hash, label := ref.Hash(), ""
		tag, err := repo.TagObject(hash)
		if err == nil {
			hash, label = tag.Target, strings.TrimSpace(tag.Message)
		} else if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return fmt.Errorf("reading %s: %w", name, err)
		}

		backup, ok := byID[id]
		if !ok {
			created, kind, _ := parseBackupID(id)
			backup = &Backup{ID: id, Kind: kind, Created: created}
			byID[id] = backup
		}
		if label != "" {
			backup.Name = label
		}
		backup.Branches = append(backup.Branches, BackupBranch{Name: branch, Hash: hash})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing refs: %w", err)
	}

	backups := make([]Backup, 0, len(byID))
	for _, backup := range byID {
		sort.Slice(backup.Branches, func(i, j int) bool { return backup.Branches[i].Name < backup.Branches[j].Name })
		backups = append(backups, *backup)
	}
	sort.Slice(backups, func(i, j int) bool {
		if !backups[i].Created.Equal(backups[j].Created) {
			return backups[i].Created.After(backups[j].Created)
		}
		return backups[i].ID > backups[j].ID
	})
	return backups, nil
}

// FindBackup returns the backup with the given ID, or ErrBackupNotFound.
func FindBackup(repo *gogit.Repository, id string) (Backup, error) {
	backups, err := ListBackups(repo)
	if err != nil {
		return Backup{}, err
	}
	for _, backup := range backups {
		if backup.ID == id {
			return backup, nil
		}
	}
	return Backup{}, fmt.Errorf("%w: %s", ErrBackupNotFound, id)
}

// SetBackupName gives the backup with the given ID a name, or removes it when
// name is empty. The name is stored in the backup itself: each of its refs is
// pointed at an annotated tag object whose message is the name and whose
// target is the saved commit, so it survives `git gc` and bundle exports, and
// git peels the tag wherever a commit is expected.
//
// Names are trimmed, must be a single line without control characters, and
// may be at most maxBackupNameLength characters (ErrInvalidBackupName).
func SetBackupName(state *RepoState, id, name string) error {
	name = strings.TrimSpace(name)
	if err := validateBackupName(name); err != nil {
		return err
	}
	backup, err := FindBackup(state.Repo, id)
	if err != nil {
		return err
	}

	var tagger object.Signature
	if name != "" {
		identity, _ := ConfiguredIdentity(state)
		tagger = object.Signature{Name: identity.Name, Email: identity.Email, When: time.Now()}
		if tagger.Name == "" {
			tagger.Name = "GitGo"
		}
	}

	for _, branch := range backup.Branches {
		target := branch.Hash
		if name != "" {
			tag := &object.Tag{
				Name:       backupTagName,
				Tagger:     tagger,
				Message:    name + "\n",
				TargetType: plumbing.CommitObject,
				Target:     branch.Hash,
			}
			encoded := state.Repo.Storer.NewEncodedObject()
			if err := tag.Encode(encoded); err != nil {
				return fmt.Errorf("encoding backup name: %w", err)
			}
			target, err = state.Repo.Storer.SetEncodedObject(encoded)
			if err != nil {
				return fmt.Errorf("writing backup name: %w", err)
			}
		}
		ref := plumbing.NewHashReference(backupRefName(id, branch.Name), target)
		if err := state.Repo.Storer.SetReference(ref); err != nil {
			return fmt.Errorf("writing backup ref: %w", err)
		}
	}
	return nil
}

// validateBackupName checks a trimmed backup name.
func validateBackupName(name string) error {
	if utf8.RuneCountInString(name) > maxBackupNameLength {
		return fmt.Errorf("%w: at most %d characters", ErrInvalidBackupName, maxBackupNameLength)
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return fmt.Errorf("%w: it must be a single line", ErrInvalidBackupName)
		}
	}
	return nil
}

// DeleteBackup removes every ref of the backup with the given ID. Its commits
// are left to `git gc` once nothing else refers to them.
func DeleteBackup(repo *gogit.Repository, id string) error {
	backup, err := FindBackup(repo, id)
	if err != nil {
		return err
	}
	for _, branch := range backup.Branches {
		if err := repo.Storer.RemoveReference(backupRefName(id, branch.Name)); err != nil {
			return fmt.Errorf("deleting backup ref: %w", err)
		}
	}
	return nil
}

// PruneBackups keeps the newest keep automatic backups of branch and removes
// the branch from older ones. Other branches in those backups are kept, and so
// are manual backups. It returns how many refs were removed.
func PruneBackups(repo *gogit.Repository, branch string, keep int) (int, error) {
	if keep < 0 {
		keep = 0
	}
	backups, err := ListBackups(repo)
	if err != nil {
		return 0, err
	}
	removed := 0
	kept := 0
	for _, backup := range backups { // newest first
		if backup.Kind != BackupAuto {
			continue
		}
		if _, ok := backup.Tip(branch); !ok {
			continue
		}
		if kept < keep {
			kept++
			continue
		}
		if err := repo.Storer.RemoveReference(backupRefName(backup.ID, branch)); err != nil {
			return removed, fmt.Errorf("deleting backup ref: %w", err)
		}
		removed++
	}
	return removed, nil
}

// RestoreCommit is a commit listed in a RestorePlan.
type RestoreCommit struct {
	Hash    plumbing.Hash
	Subject string
}

// RestoreBranchPlan says what restoring a backup does to one branch.
type RestoreBranchPlan struct {
	Branch     string
	BackupTip  plumbing.Hash
	CurrentTip plumbing.Hash
	// Missing is true when the branch has been deleted since the backup. It is
	// skipped: a restore only moves existing branches.
	Missing bool
	// CheckedOut is true when the branch is the one HEAD points at.
	CheckedOut bool
	// Removed lists commits on the branch now that the restore takes off it
	// (newest first, at most maxRestoreListed); RemovedCount counts them all.
	Removed      []RestoreCommit
	RemovedCount int
	// Returned lists the backup's commits the restore brings back, in the
	// same way.
	Returned      []RestoreCommit
	ReturnedCount int
	// Blocked is why this branch cannot be restored (ErrRestorePushed or
	// ErrRestoreFilesChanged), or nil.
	Blocked error
}

// Unchanged reports whether the branch already points at the backup's commit.
func (plan RestoreBranchPlan) Unchanged() bool {
	return !plan.Missing && plan.CurrentTip == plan.BackupTip
}

// RestorePlan describes what restoring a backup would do, for the confirm
// dialog, and is what RestoreBackup checks before moving anything.
type RestorePlan struct {
	Backup   Backup
	Branches []RestoreBranchPlan
}

// Err returns the first reason the restore cannot go ahead, or nil.
func (plan RestorePlan) Err() error {
	for _, branch := range plan.Branches {
		if branch.Blocked != nil {
			return branch.Blocked
		}
	}
	return nil
}

// PlanRestore works out what restoring the backup with the given ID would do to
// each of its branches in the repository at path.
//
// A branch is blocked when the restore would remove a commit that has been
// pushed (ErrRestorePushed), or when the branch is checked out and its files
// differ from the backup's (ErrRestoreFilesChanged): moving the branch would
// then leave the difference behind as uncommitted changes.
func PlanRestore(path, id string) (RestorePlan, error) {
	state, err := OpenBranch(path, "")
	if err != nil {
		return RestorePlan{}, err
	}
	backup, err := FindBackup(state.Repo, id)
	if err != nil {
		return RestorePlan{}, err
	}

	plan := RestorePlan{Backup: backup}
	for _, saved := range backup.Branches {
		branchPlan, err := planBranchRestore(path, saved)
		if err != nil {
			return RestorePlan{}, err
		}
		plan.Branches = append(plan.Branches, branchPlan)
	}
	return plan, nil
}

// planBranchRestore builds the RestoreBranchPlan for one saved branch.
func planBranchRestore(path string, saved BackupBranch) (RestoreBranchPlan, error) {
	plan := RestoreBranchPlan{Branch: saved.Name, BackupTip: saved.Hash}

	state, err := OpenBranch(path, saved.Name)
	if errors.Is(err, ErrBranchNotFound) {
		plan.Missing = true
		return plan, nil
	}
	if err != nil {
		return plan, err
	}
	tip, err := branchTip(state)
	if err != nil {
		return plan, err
	}
	plan.CurrentTip = tip.Hash()
	plan.CheckedOut = state.IsCheckedOut
	if plan.Unchanged() {
		return plan, nil
	}

	backupCommit, err := state.Repo.CommitObject(saved.Hash)
	if err != nil {
		return plan, fmt.Errorf("loading commit %s: %w", saved.Hash, err)
	}
	currentCommit, err := state.Repo.CommitObject(plan.CurrentTip)
	if err != nil {
		return plan, fmt.Errorf("loading commit %s: %w", plan.CurrentTip, err)
	}

	removed, err := commitsOnlyIn(state, plan.CurrentTip, saved.Hash)
	if err != nil {
		return plan, err
	}
	returned, err := commitsOnlyIn(state, saved.Hash, plan.CurrentTip)
	if err != nil {
		return plan, err
	}
	plan.RemovedCount = len(removed)
	plan.ReturnedCount = len(returned)
	plan.Removed = listRestoreCommits(state, removed)
	plan.Returned = listRestoreCommits(state, returned)

	for hash := range removed {
		if !state.UnpushedHashes[hash] {
			plan.Blocked = fmt.Errorf("%w: %s", ErrRestorePushed, saved.Name)
			return plan, nil
		}
	}
	if plan.CheckedOut && currentCommit.TreeHash != backupCommit.TreeHash {
		plan.Blocked = fmt.Errorf("%w: %s", ErrRestoreFilesChanged, saved.Name)
	}
	return plan, nil
}

// commitsOnlyIn returns the commits reachable from tip but not from other.
func commitsOnlyIn(state *RepoState, tip, other plumbing.Hash) (map[plumbing.Hash]bool, error) {
	kept, err := reachableFrom(state.Repo, []plumbing.Hash{other}, nil)
	if err != nil {
		return nil, fmt.Errorf("walking history: %w", err)
	}
	only, err := reachableFrom(state.Repo, []plumbing.Hash{tip}, kept)
	if err != nil {
		return nil, fmt.Errorf("walking history: %w", err)
	}
	return only, nil
}

// listRestoreCommits returns up to maxRestoreListed of the given commits,
// newest first.
func listRestoreCommits(state *RepoState, hashes map[plumbing.Hash]bool) []RestoreCommit {
	type dated struct {
		commit RestoreCommit
		when   time.Time
	}
	all := make([]dated, 0, len(hashes))
	for hash := range hashes {
		commit, err := state.Repo.CommitObject(hash)
		if err != nil {
			continue
		}
		subject, _, _ := strings.Cut(commit.Message, "\n")
		all = append(all, dated{RestoreCommit{Hash: hash, Subject: subject}, commit.Committer.When})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].when.Equal(all[j].when) {
			return all[i].when.After(all[j].when)
		}
		return all[i].commit.Hash.String() < all[j].commit.Hash.String()
	})
	if len(all) > maxRestoreListed {
		all = all[:maxRestoreListed]
	}
	listed := make([]RestoreCommit, len(all))
	for i, entry := range all {
		listed[i] = entry.commit
	}
	return listed
}

// RestoreBackup moves each branch of the backup with the given ID back to the
// commit the backup saved. expected maps each branch to the tip the caller
// planned with (see PlanRestore); if any branch has moved since, nothing is
// changed and ErrBranchChanged is returned. Nothing is changed either when the
// plan is blocked (see RestorePlan.Err).
//
// Branches deleted since the backup, and ones already at the backup's commit,
// are skipped. Each move is a compare-and-swap recorded in the reflog as
// "gitgo: restore backup <id>". The returned plan is the one that was carried
// out, also when moving a branch fails after the checks passed: the error then
// names the branches that were not moved, and the others stay restored.
func RestoreBackup(path, id string, expected map[string]plumbing.Hash) (RestorePlan, error) {
	plan, err := PlanRestore(path, id)
	if err != nil {
		return RestorePlan{}, err
	}
	for _, branch := range plan.Branches {
		if branch.Missing || branch.Unchanged() {
			continue
		}
		if want, ok := expected[branch.Branch]; !ok || want != branch.CurrentTip {
			return RestorePlan{}, ErrBranchChanged
		}
	}
	if err := plan.Err(); err != nil {
		return RestorePlan{}, err
	}

	var notMoved []string
	var moveErr error
	for _, branch := range plan.Branches {
		if branch.Missing || branch.Unchanged() {
			continue
		}
		state, err := OpenBranch(path, branch.Branch)
		if err == nil {
			err = moveBranch(state, plumbing.NewBranchReferenceName(branch.Branch), branch.CurrentTip, branch.BackupTip, reflogRestorePrefix+id)
		}
		if err != nil {
			notMoved = append(notMoved, branch.Branch)
			moveErr = err
		}
	}
	if len(notMoved) > 0 {
		return plan, fmt.Errorf("could not restore branch %s: %w", strings.Join(notMoved, ", "), moveErr)
	}
	return plan, nil
}
