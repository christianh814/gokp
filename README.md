# gokp
`gokp` aims to install a GitOps Native Kubernetes Platform.

This project is a Proof of Concept centered around getting a GitOps
aware Kubernetes Platform on Day 0. The installer aims to:

* Install an HA Kubernetes cluster (AWS or Docker)
* Install the chosen GitOps controller (Argo CD or Flux CD)
* Configure the chosen GitOps controller in an opinionated way
* Export all YAML into a Git repo (GitHub only currently)
* Deliver a "ready to go with GitOps" cluster.

The idea being that the end user just needs to start commiting to the
proper directory to futher confgure the cluster. GitOps ready, from the
get go!

Please keep in mind that this is a PoC and should be considered Pre-Pre-Alpha.

Take a look at the [Documentation Repo](https://github.com/christianh814/gokp-documentation) for more info.
