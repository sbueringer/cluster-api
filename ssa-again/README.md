# README

Some commands:

```bash
kubectl --kubeconfig /tmp/kubeconfig --server=https://127.0.0.1:20000 apply  -f ./test-machine-1-taints-v1beta1.yaml --server-side=true --field-manager=capi-machineset
kubectl --kubeconfig /tmp/kubeconfig --server=https://127.0.0.1:20000 label machines.v1beta1.cluster.x-k8s.io abc=def


w "kubectl --kubeconfig /tmp/kubeconfig --server=https://127.0.0.1:20000 get machine -n default default -o yaml --show-managed-fields| grep -i key"
```
