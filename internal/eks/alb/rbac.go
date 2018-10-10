package alb

const albYAMLRBAC = `---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole

metadata:
  labels:
    app: alb-ingress-controller
  name: alb-ingress-controller

rules:
  - apiGroups:
      - ""
      - extensions
    resources:
      - configmaps
      - endpoints
      - events
      - ingresses
      - ingresses/status
      - services
    verbs:
      - create
      - get
      - list
      - update
      - watch
      - patch
  - apiGroups:
      - ""
      - extensions
    resources:
      - nodes
      - pods
      - secrets
      - services
      - namespaces
    verbs:
      - get
      - list
      - watch

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding

metadata:
  labels:
    app: alb-ingress-controller
  name: alb-ingress-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: alb-ingress-controller
subjects:
  - kind: ServiceAccount
    name: alb-ingress
    namespace: kube-system

---
apiVersion: v1
kind: ServiceAccount

metadata:
  labels:
    app: alb-ingress-controller
  name: alb-ingress
  namespace: kube-system

`
