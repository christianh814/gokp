#!/bin/bash

#
## Directory to export to as first argument
exportdir=${1}

#
## Exit if dir not there
if [[ ! -d ${exportdir:="NODIR"} ]]; then
	echo "Directory doesn't exist or not provided"
	exit 13
fi

#
## First, figure out if the right files are in place, if not exit.
for file in kubectl-export yq kubectl
do
	if ! which ${file} >/dev/null 2>&1 ; then
		echo "Required ${file} not found in your PATH"
		exit 13
	fi
done

#
## Check to see if exported dir has a cluster dir already.
##	TODO: bult in a "overwrite" function
if [[ -d ${exportdir}/cluster ]]; then
	echo "${exportdir}/cluster Exits! Manual intervention required"
	exit 13
fi

#
## create template files
cat<<EOF > /tmp/k.cluster.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization


commonAnnotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
    argocd.argoproj.io/sync-options: Validate=false

resources:
EOF

cat<<EOF > /tmp/k.deftemp.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization


commonAnnotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true

resources:
EOF

#
## Create the cluster dir
echo "Creating ${exportdir}/cluster"
mkdir ${exportdir}/cluster

#
## Export all cluster scoped API Resources
for cc in $(kubectl api-resources  --namespaced=false -o name)
do
	#
	## Skip things that cannot/shouldnot be exported
	[[ ${cc} == "componentstatuses" ]] || [[ ${cc} == "namespaces" ]] [[ ${cc} == "certificatesigningrequests" ]] && continue

	#
	## Skip if resource isn't found
	[[ $(kubectl get ${cc} 2>&1 | grep -c 'No resources found') -ne 0 ]] && continue

	#
	## Export every cluster component found into exportdir
	kubectl get ${cc} -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | while read sc
	do
		# Clean subcomponent if it needs it
		nsc=$(echo ${sc} | sed -e 's/:/-/g')
		kubectl export ${cc} ${sc} > ${exportdir}/cluster/${cc}-${nsc}.yaml
	done
done

#
## Export namespaced components
for nc in $(kubectl api-resources  --namespaced=true -o name)
do
	#
	## Get each namespaced component from each  namespace
	for ns in $(kubectl get ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
	do
		#
		## Create the NS directory and export the namespace YAML
		mkdir -p ${exportdir}/${ns}
		kubectl export ns ${ns} > ${exportdir}/${ns}/${ns}-namespace.yaml

		#
		## Skip things we don't want/need
		[[ ${nc} == "bindings" ]] || \
		[[ ${nc} == "pods" ]] || \
		[[ ${nc} == "endpoints" ]] || \
		[[ ${nc} == "replicasets.apps" ]] || \
		[[ ${nc} == "localsubjectaccessreviews.authorization.k8s.io" ]] || \
		[[ ${nc} == "endpointslices.discovery.k8s.io" ]] && continue

		#
		## Don't bother exporting things that aren't there
		[[ $(kubectl get ${nc} -n ${ns} 2>&1 | grep -c 'No resources found') -ne 0 ]] && continue

		#
		## Export each object from the objects found in the namespace
		for obj in $(kubectl get ${nc} -n ${ns} -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
		do
			nobj=$(echo ${obj} | sed -e 's/:/-/g')
			kubectl export ${nc} ${obj} -n ${ns} > ${exportdir}/${ns}/${nc}-${nobj}.yaml
		done
	done
done


#
## Create the kustomize file in each dir
for dir in $(ls -1 ${exportdir})
do
	#
	## if it's the cluster dir...we need to use that specifc one
	if [[ ${dir} == "cluster" ]] ; then
		outfile=/tmp/k.cluster.yaml
	else
		outfile=/tmp/k.deftemp.yaml
	fi

	#
	## Write the files into that kustomization.yaml
	for file in $(ls -1 ${exportdir}/${dir})
	do
		echo "- ${file}" >> ${outfile}
	done
	mv ${outfile} ${exportdir}/${dir}/kustomization.yaml
done

##
##
