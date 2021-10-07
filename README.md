# gokp
`gokp` aims to install a GitOps Native Kubernetes Platform.

This project is a Proof of Concept centered around getting a GitOps
aware Kubernetes Platform on Day 0. The installer aims to:

* Install an HA Kubernetes cluster (AWS only currently)
* Install Argo CD
* Configure Arog CD in an opinionated way
* Export all YAML into a Git repo (GitHub only currently)
* Deliver a "ready to go with GitOps" cluster.

The idea being that the end user just needs to start commiting to the
proper directory to futher confgure the cluster. GitOps ready, from the
get go!

Please keep in mind that this is a PoC and should be considered Pre-Pre-Alpha.

# Prerequisites

The following are preqs. Since this is centered around [KIND](https://kind.sigs.k8s.io/) and [CAPI](https://cluster-api.sigs.k8s.io/), there will be a lot of similar prereqs:

__Absolutley Needed__

These are things that are needed:

* AWS Account
* GitHub Token
* Docker on your workstation

Podman may or maynot work. It's [considered experemental by KIND](https://kind.sigs.k8s.io/docs/user/rootless/#creating-a-kind-cluster-with-rootless-podman) so YMVM.


__Nice to Haves__

The following aren't needed, but are nice to have

* Kubernetes CLI (`kubectl`)
* Cluster API CLI (`clusterctl`)
* Cluster API AWS CLI (`clusterawsadm`)
* AWS CLI (`aws`)
* KIND CLI (`kind`)

__More Info__

For more information around the Prereqs, please see the documentation for [KIND](https://kind.sigs.k8s.io/) and [CAPI](https://cluster-api.sigs.k8s.io/). IAM info can be found [HERE](https://cluster-api-aws.sigs.k8s.io/topics/iam-permissions.html#ec2-provisioned-kubernetes-clusters)

# Testing

I've hand tested this on Ubuntu 21.04, Fedora 34, and Mac OS X Big Sur (11.6) all on X86_64.

I currently don't have any binaries, so you will need to install this via golang (I'm using 1.17.1):

```shell
go install github.com/christianh814/gokp
```

If you don't have your `GOBIN` in your `PATH`, you need to set that

```shell
export PATH=$GOBIN:$PATH
```

Now you can get bash completion

```shell
source <(gokp completion bash)
```

Currently only one command exists, `create-cluster`. You will need to
tell it certian things like you AWS credentials and your GitHub token (you
need to have a token that has `Full control of private repositories`).

> NOTE: If you choose to connect to a privte repo, then I commit that
> token to your repo. If you don't want this then use a public repo.

Here's an example of a command run. I've exported my params into my ENV, so substitue your information where applicable.

```shell
gokp create-cluster --cluster-name=$MYCLUSER \
--github-token=$GH_TOKEN \
--aws-ssh-key=$AWS_SSH_KEY_NAME \
--aws-access-key=$AWS_ACCESS_KEY_ID \
--aws-secret-key=$AWS_SECRET_ACCESS_KEY \
--private-repo=true
```

> NOTE: The aws ssh key must reference a key you've already uploaded. See [this doc](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/ec2/import-key-pair.html) for more info.

This will spin up a temporary control plane, locally, using KIND. Then it will use KIND to install a cluster on on AWS. It will then create a Git repo for the cluster. Once the AWS cluster is ready, the installer will install Argo CD in an opinionated way and then deploy the Git repo it created.

You should see something like this:

```shell
INFO[1037] Cluster Successfully installed! Everything you need is under: ~/.gokp/$MYCLUSTER
```

Your Kubernetes cluster is ready. Take a look 

```shell
export KUBECONFIG=~/.gokp/$MYCLUSTER/$MYCLUSTER.kubeconfig
```

Run `kubectl get pods -A` and you should see ArgoCD pods along with a Kuard smaple application.

```
$ k get pods -A | egrep 'argocd|kuard'
NAMESPACE     NAME                                                  READY   STATUS    RESTARTS       AGE
argocd        argocd-application-controller-0                       1/1     Running   0              4m37s
argocd        argocd-applicationset-controller-5856984b4c-wj8s8     1/1     Running   0              4m39s
argocd        argocd-dex-server-64f95f8c77-2wzcj                    1/1     Running   0              4m39s
argocd        argocd-redis-978c79c75-l9c6p                          1/1     Running   0              4m39s
argocd        argocd-repo-server-85d6c7c8c4-jqwdz                   1/1     Running   0              4m39s
argocd        argocd-server-7f86787cf-s5sh9                         1/1     Running   0              4m39s
kuard         kuard-857f95f9df-99x87                                1/1     Running   0              4m43s
```

Run `kubectl get nodes` and you should have 3 controlplane and 3 worker nodes.

```
$ kubectl get nodes
NAME                          STATUS   ROLES                  AGE   VERSION
ip-10-0-155-27.ec2.internal   Ready    control-plane,master   11m   v1.22.2
ip-10-0-254-40.ec2.internal   Ready    control-plane,master   12m   v1.22.2
ip-10-0-69-120.ec2.internal   Ready    <none>                 12m   v1.22.2
ip-10-0-71-144.ec2.internal   Ready    control-plane,master   10m   v1.22.2
ip-10-0-77-205.ec2.internal   Ready    <none>                 12m   v1.22.2
ip-10-0-85-76.ec2.internal    Ready    <none>                 12m   v1.22.2
```

Login to your Argo CD instance

Get the inital admin password

```shell
kubectl get secrets argocd-initial-admin-secret -n argocd  -o jsonpath='{.data.password}' | base64 -d ; echo
```

Then port-forward to access it

```shell
kubectl port-forward service/argocd-server 8080:443 -n argocd
```

You should be able to access the UI by visiting `http://127.0.0.1:8080`. There you'll see two `Applications` one for `argocd` itself and another for `kuard`

This was deployed as `ApplicationSets`, take a look.

```shell
kubectl get appsets -n argocd
```

Which then creates `Applications`

```shell
kubectl get apps -n argocd
```

This is deployed via the Git repo it created, so check your account. Any changes you want to make should be done via the Git repo.

# Clean Up

WHATEVER YOU DO. Don't delete your KIND cluster (locally). If you do, there's no way to clean up the cluster you just created. You're on your own. I suggest visiting the Cluster API Slack channel and ask around.

To delete the AWS Cluster, first export that KUBECONFIG file

```shell
export KUBECONFIG=~/.gokp/$MYCLUSTER/kind.kubeconfig
```

Now delete your cluster instance (or run `kubectl delete clusters --all`)

```shell
kubectl delete clusters $MYCLUSTER
```

This will take a bit, once that's done you can delete the KIND instance (called `gokp-bootstrapper`)

```shell
kind delete cluster --name gokp-bootstrapper
```

> If you don't have the `kind` CLI installed, just run `docker ps` to get the name and run `docker stop` on it.

# Questions?

Any questions, I can be found on the [Kubernetes Slack](https://slack.k8s.io/), I am the user `@christianh814`.
