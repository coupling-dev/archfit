# Try it on your pull request

In [Pumba](https://github.com/alexei-led/pumba), a change added a dependency and [the check failed](https://github.com/alexei-led/pumba/pull/414). The author [requested it](https://github.com/alexei-led/pumba/issues/415), and the owner [answered](https://github.com/alexei-led/pumba/pull/418) with one line in the policy. The answer is remembered for the next change.

## Set up advisory checks

1. Install the CLI. On macOS or Linux with Homebrew:

   ```sh
   brew install coupling-dev/tap/archfit
   ```

   On Linux without Homebrew, download the binary for your architecture from the [releases page](https://github.com/coupling-dev/archfit/releases), verify it against `SHA256SUMS`, and put it on your `PATH`.
2. From the default branch of a checkout with a GitHub `origin`, run `archfit setup`. Read the policy and follow each `next:` line, running `archfit setup` again when asked. Setup writes the workflow and opens a setup pull request, or prints a compare link to open one yourself.
3. Install the App for the repository from the link printed by setup. If the check does not start, use the printed re-run link.
4. Review and merge the setup pull request. Run `archfit setup` again on the default branch:

   ```text
   advisory: checks report on each pull request and do not block a merge
   state: advisory checks on
   next: open the setup page: https://app.coupling.dev
   an owner approves the first policy there, then makes the check required
   ```

   An owner approves the first policy on the setup page before anyone can answer a request. When the policy has `gate: required`, setup says `required checks on` and asks you to check that a branch ruleset requires `archfit/gate`: only the ruleset makes a fail block a merge.

No sign-in or passkey is required for initial advisory checks. See the [setup command reference](commands.md#setup) for its states and recovery steps.

## What to expect

- The setup pull request shows a neutral check until it is merged. After the merge, checks are advisory, not required.
- On a later pull request, a change that follows the policy can pass; a change that breaks a rule reports the problem and its next step. An owner can answer a request for an exception.

Fork pull requests cannot upload the record: GitHub does not grant them the `id-token: write` permission needed by the Action. They are not supported by the required check. See [CI](ci.md) for a local check.

## How to leave

Uninstall the Archfit App from the repository, then delete `.archfit.yaml` and `.github/workflows/archfit.yaml` in a pull request. Setup may also have added agent instructions, hooks and a `.gitignore` entry; review and remove those separately if you no longer want them.
