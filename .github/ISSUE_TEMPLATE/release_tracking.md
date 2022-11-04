---
name: 🚋 Release cycle tracking
about: Create a new release cycle tracking issue for a minor release
title: Tasks for v<release-tag> release cycle
labels: ''
assignees: ''

---

Please see the corresponding section in [release-tasks.md](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/release/release-tasks.md) for documentation of individual tasks.  

## Tasks

**Notes**:
* Weeks are only specified to give some orientation.
* The following is based on the v1.4 release cycle. Modify according to the tracked release cycle.

Week -3 to 1:
* [ ] [Release Lead] Set a tentative release date for the minor release
* [ ] [Release Lead] Assemble release team

Week 1:
* [ ] [Release Lead] Finalize release schedule and team
* [ ] [Release Lead] Prepare main branch for development of the new release
* [ ] [Release Lead] Create a new GitHub milestone
* [ ] [Communications Manager] Add docs to collect release notes for users and migration notes for provider implementers
* [ ] [Communications Manager] Update supported versions

Week 1 to 4:
* [ ] [Release Lead] [Track] Remove previously deprecated code

Week 6:
* [ ] [Release Lead] Cut the v1.3.1 release

Week 9:
* [ ] [Release Lead] Cut the v1.3.2 release

Week 11 to 12:
* [ ] [Release Lead] [Track] Bump dependencies

Week 13:
* [ ] [Release Lead] Cut the v1.4.0-beta.0 release
* [ ] [Release Lead] Cut the v1.3.3 release

Week 14:
* [ ] [Release Lead] Cut the v1.4.0-beta.1 release
* [ ] [Release Lead] Select release lead for the next release cycle

Week 15:
* [ ] [Release Lead] Create the release-1.4 release branch
* [ ] [Release Lead] Cut the v1.4.0-rc.0 release
* [ ] [CI Manager] Setup jobs and dashboards for the release-1.4 release branch
* [ ] [Communications Manager] Ensure the book for the new release is available

Week 15 to 17:
* [ ] [Communications Manager] Polish release notes

Week 16:
* [ ] [Release Lead] Cut the v1.4.0-rc.1 release

Week 17:
* [ ] [Release Lead] Cut the v1.4.0 release
* [ ] [Release Lead] Cut the v1.3.4 release
* [ ] [Release Lead] Organize release retrospective
* [ ] [Communications Manager] Change production branch in Netlify to the new release branch
* [ ] [Communications Manager] Update clusterctl links in the quickstart

Continuously:
* [Release lead] Maintain the GitHub release milestone
* [Communications Manager] Communicate key dates to the community
* [CI Manager] Monitor CI signal
* [CI Manager] Reduce the amount of flaky tests
* [CI Manager] Bug triage

If and when necessary:
* [ ] [Release Lead] [Track] Bump the ClusterAPI apiVersion
* [ ] [Release Lead] [Track] Bump the Kubernetes version
