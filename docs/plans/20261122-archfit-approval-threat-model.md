# Archfit approval threat model (D6)

Status: decided by the owner on 2026-11-22 (section 7). Task 8.1 of the onboarding plan
(`docs/plans/20261122-archfit-0-10-onboarding.md`, section 6.2). Decision D6 is the owner's. No code.

Sources:

- The change-control design, `docs/plans/20261110-archfit-4-architecture-change-control.md`, sections 9 and 10.4.
- The onboarding design, `docs/plans/20261120-archfit-onboarding-design.md`, sections 3, 7 and 8 (O1 amended).
- The App, `coupling-dev/archfit-app` `main` at `591a67c`, read only. App paths below are in that repository.
- GitHub docs, read 2026-11-22: [code owners][gh-codeowners], [rulesets][gh-rulesets],
  [environments][gh-env], [reviewing deployments][gh-review-deploy], [commit signatures][gh-signing].

## 1. What an approval must prove

An approval lets the policy loosen, or starts change control at "Make it required" (onboarding design,
section 3). The required check `archfit/gate` depends on it (change-control 9.2).

| Id  | Claim                                                                                                                                                                      |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P1  | **An owner.** The approver is an owner named in the approved policy, not any account with write access.                                                                    |
| P2  | **A person, now.** A person made it at that moment. An owner's token, cookie or signing key in someone else's hands is not enough (change-control 10.4).                    |
| P3  | **This change, once.** It is bound to the exact change (policy and workflow at one head) and cannot be used again for another change.                                       |
| P4  | **A record.** Who, when, with which credential, readable later.                                                                                                             |

No option proves that the person read and understood the request (change-control 9.5).

## 2. Actors

| Actor                                 | Has                                                                                                                                                                                                | Does not have                                                                   |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| (a) Coding agent                      | The developer's GitHub token (`gh`), the checkout, the developer's signing agent (`ssh-agent`, `gpg-agent` with a cached passphrase), often the developer's browser. The developer can be an owner. | The developer's finger, face or PIN; a hardware key's touch.                    |
| (b) Developer who is not an owner     | Write access; their own token, keys and App sign-in.                                                                                                                                               | Any owner credential.                                                           |
| (c) Stolen laptop with active session | All of (a), on an owner's unlocked laptop: browser sessions (GitHub; App cookie, 8 hours), tokens, signing agents.                                                                                  | The owner's biometrics, the device password, a hardware key not left plugged in. |

Every option stops (b), because each checks the owner list. They differ on (a) and (c).

## 3. Today: the App passkey

What the App does at `591a67c`:

- **Sign-in.** GitHub OAuth. The session cookie lives 8 hours and holds the OAuth token
  (`internal/control/ui/session/cookie.go:18,68`).
- **When a passkey is needed.** Every owner action outside a request: the first approval, a workflow approval, a
  confirm (`internal/gate/approvals/owner.go:293-297`). An answer to a request only when it loosens: approve,
  approve until, confirm, declare public, declare model; a reject does not (`internal/gate/proposals/proposals.go:358-367`,
  `internal/wire/policy3/answer.go:30-38`). Edits and reverts (`proposals/edit.go:133`, `proposals/revert.go:112`).
- **Who.** A first approval: a repository administrator who is also an owner in the policy being approved. Any other
  action: an owner named in the approved policy, read at the approved commit (`approvals/owner.go:325-368`). A team
  entry matches nobody (`approvals/texts.go:94-98`).
- **User verification** is required at registration and at each step-up, and checked on the response
  (`internal/control/passkey/passkey.go:83,257,312`). Attestation is `none` (`passkey.go:79`).
- **Binding.** The step-up token is bound to the repository, the action and the change digest; it is used once and
  expires after 5 minutes (`passkey/contract.go:22`, `passkey.go:468-480`). The digest covers the pull request, its
  head, and the policy and workflow digests; a moved head is refused (`approvals/owner.go:59-86,283-285`).
- **Record.** Owner, login, credential and time, noted on the pull request or an issue (`approvals/owner.go:303-308,434-454`).
- **Outside the flow.** A policy change on the default branch with no recorded approval makes results `unknown`
  until an owner confirms (`internal/gate/upload_result.go:50`, `internal/gate/home.go:152-154`).
- **Registration.** A user's first passkey needs only a signed-in session of an owner (or of an administrator before
  the first policy). A second passkey needs a step-up with an existing one. Each registration opens an issue assigned
  to the owners (`passkey.go:129-137,221,425-440`).

**Gap G1: the first registration.** Whoever holds an owner's App session before that owner has a passkey can
enroll one: actor (a) through a browser it drives, actor (c) with a security key of their own. With attestation
`none`, the App cannot tell a real authenticator from a virtual one that browser automation adds and that reports
user verification (Chrome DevTools Protocol, `WebAuthn.addVirtualAuthenticator` with `isUserVerified`; documented,
not tried here, so 8.9 tries it before it picks a fix). The registration issue tells the owners afterwards; it does
not stop it.

**Gap G2: the rung 2 window.** Before "Make it required" no approval exists: an owner merges policy pull requests on
GitHub with no passkey (onboarding design, O1). Actors (a) and (c) can loosen the policy in that window, and the first
approval then covers it, so P3 does not hold for those changes. The amended O1 is the mitigation: the "Make it
required" page lists each policy change since the setup pull request, with who merged it and its request. G2 is the
same for every option below.

**Assumption: user verification is the authenticator's.** The App checks the flag, not how it was set
(`passkey.go:312`). A biometric or a hardware key's PIN holds against (c). A passkey that a password manager or the
operating system unlocks with the device password holds only while the thief does not know that password.

## 4. Options

| Option                                 | P2 (a person) | Stops (a)      | Stops (b) | Stops (c)                      | Team setup                                                          | 4-step route                         |
| -------------------------------------- | ------------- | -------------- | --------- | ------------------------------ | ------------------------------------------------------------------- | ------------------------------------ |
| 1. App passkey (today)                 | Yes, UV       | Yes, except G1 | Yes       | Yes, except G1 and weak UV     | One passkey per owner. Built.                                       | No change                            |
| 2a. Signed policy commit, software key | No            | No             | Yes       | No                             | Signing per owner, pinned key list, merge commits, new verifier     | No change                            |
| 2b. Signed policy commit, hardware key | Yes, touch    | Yes            | Yes       | Yes, unless key and PIN at hand | As 2a, and a hardware key per owner                                 | No change                            |
| 3. GitHub code-owner review            | No            | No             | Yes       | No                             | CODEOWNERS and a ruleset; private: Pro, Team or Enterprise          | Setup pull request gains CODEOWNERS  |
| 4. GitHub environment approval         | No            | No             | Yes       | No                             | Environment and a workflow job; private: Enterprise only            | The workflow `setup` writes changes  |

Steps 1 to 4 carry no approval (decision D2, onboarding design 7). Options 1, 2a and 2b add work only at "Make it
required"; options 3 and 4 also change what `setup` writes.

**1. App passkey.** Proves P1 to P4 (section 3). Does not prove that the first passkey belongs to the owner (G1).
"Make it required" carries the sign-in, the passkey and the first approval (onboarding design 3). The owner needs a
device with a passkey.

**2a. Signed commit, software key.** Proves P1 against a key list pinned by Archfit, P3 (the signature covers the
tree), and P4 offline: the CLI could verify without the App (change-control 9.3). Does not prove P2: the key sits
in an agent that signs silently. GitHub's "Verified" badge is no proof: GitHub signs commits made in its web UI with
its own key, and a squash merge replaces the owner's signature with GitHub's ([commit signatures][gh-signing]). So
the key list cannot be GitHub's account keys, the repository must keep the owner's commit (merge commits) or the
App must check the head before merge, and the key list needs the rules of change-control 9.3: a version that never
decreases, a link to the previous one, an expiry. Each owner sets up signing on each machine and keeps keys rotated.

**2b. Signed commit, hardware key with touch** (an `sk` SSH key, or an OpenPGP card with a touch policy). As 2a,
and P2 at the level of a touch (a PIN too with `verify-required`). Weaker than 1 on what the person sees: the key
signs whatever git asks, and the touch shows no digest; an agent can ask at the moment the owner expects a touch
for something else. Each owner buys a hardware key and a spare.

**3. Code-owner review.** CODEOWNERS for `.archfit.yaml` and the workflow, and "require review from code owners"
([code owners][gh-codeowners], [rulesets][gh-rulesets]). Proves P1 (an account in CODEOWNERS approved), P3 when
stale approvals are dismissed, P4. Does not prove P2: a review made with an owner's token is the owner's
(change-control 10.4). GitHub forbids approving one's own pull request. The App opens the policy pull request, so
an agent with an owner's token can approve it, and a single owner can too. "Approve and merge" becomes two actions, because the
App must not be a bypass actor (change-control 9.2).

**4. Environment approval.** A job in the workflow uses an environment with required reviewers; the App checks that
an owner approved the run on that head. Proves P1, P3, P4. Does not prove P2: an approval is one REST call,
`POST /repos/{owner}/{repo}/actions/runs/{run_id}/pending_deployments`, with a `repo`-scoped token
([reviewing deployments][gh-review-deploy]). Required reviewers on private repositories need Enterprise
([environments][gh-env]). The owner needs Enterprise for private repositories.

## 5. Recommendation

**Keep option 1, the App passkey, for 0.10.0.** It is the only option, besides 2b, that stops actor (a). It is
built and tested, adds no step to the route, and needs no GitHub plan and no hardware.

- **Close G1 in 8.9**, as an input to its proposal. This page does not choose how.
- **Record 2b as the later, offline-checked mode** under change-control 9.3. Not in 0.10.0: it costs every team
  hardware and a merge-method change, and no partner asked for it.
- **Do not use 3 or 4 as the approval:** a token alone produces them, so they do not stop actor (a). A team can
  still require code-owner reviews for its own reasons; Archfit does not rely on them.

## 6. Questions for the owner

| Id    | Question                                                                                          | Default                                      |
| ----- | ------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| D6    | Which approval does "Make it required" use in 0.10.0?                                             | Option 1, the App passkey.                    |
| D6-G1 | Close G1 in 8.9, or accept it for the beta with the registration issue as the only notice?        | Close it in 8.9; the 8.9 proposal picks how.  |
| D6-G2 | Is the amended O1 listing enough for the rung 2 window, or must 8.9 do more?                     | The listing is enough for the beta.           |
| D6-2b | Put 2b on the roadmap after 0.10.0, as an alternative to the passkey or as its replacement?       | Roadmap, as an alternative; no date.          |


## 7. Owner decisions (2026-11-22)

| Id | Decision |
| --- | --- |
| D6 | Option 1, the App passkey, for "Make it required" in 0.10.0. |
| D6-G1 | Closed in task 8.9. The 8.9 proposal picks how (for example a GitHub re-authentication or a one-time code to the owner) and says why it stops an agent that drives a signed-in browser. |
| D6-G2 | The amended O1 listing (every policy change since setup, who merged it, its request) is enough for the beta. |
| D6-2b | Hardware-key signed policy commits go on the roadmap after 0.10.0, as an alternative to the passkey, with no date. |

[gh-codeowners]: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners
[gh-rulesets]: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets
[gh-env]: https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments
[gh-review-deploy]: https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/review-deployments
[gh-signing]: https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification
