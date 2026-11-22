# Prometheus のセットアップ

このディレクトリでは、`prometheus-community/prometheus` Helm chart を Helmfile から導入します。Istio proxy の通信メトリクス、Node Exporter のノード指標、kube-state-metrics の Kubernetes リソース状態、Go アプリケーションの HTTP 指標を収集します。Prometheus のデータは `monitoring` namespace に保存します。

## dev と prod の設定

Helmfile の `-e dev` と `-e prod` は、設定ファイルを選択します。接続先クラスタは `kubectl` の現在の context で決まります。どちらも同じ `prometheus` release を更新するため、dev と prod の Prometheus を同時に作るのではなく、選んだ設定で同じインスタンスを更新します。

| 設定 | dev | prod |
|---|---|---|
| 保存期間 | 24時間 | 2日 |
| データ保存 | 2Gi の `emptyDir`。Pod 再起動で消える | 5Gi PVC |
| CPU / メモリ | CPU 100m・メモリ 128Mi を要求、メモリ上限 512Mi | CPU 500m・メモリ 1Gi を要求、メモリ上限 2Gi |
| Node CPU アラート閾値 | 50% 超 | 80% 超 |

`prod` は学習用の設定例です。PVC を作成できる StorageClass がクラスタに必要です。適用前に `kubectl get storageclass` で確認してください。認証や TLS、冗長化などはこのサンプルでは設定していません。

## Node CPU アラート

Node Exporter の `node_cpu_seconds_total` から、ノードごとのCPU使用率を計算する `NodeCpuUsageHigh` ルールを設定しています。5分窓で計算したCPU使用率が dev では50%、prod では80%を超え、その状態がさらに5分続くと Prometheus のアラート状態が `Firing` になります。ルールは環境別の values に置き、閾値を分けています。

Prometheus chart の既定設定で `alerting_rules.yml` が読み込まれます。Alertmanager はまだ無効なので、通知は送られません。適用後、Prometheus UI の <http://localhost:9090/alerts> でルールの状態を確認します。

## 前提条件

- `mise install` 済みで、Helm と Helmfile が使えること
- `helm-diff` plugin がインストール済みであること。手順は [Argo CD のセットアップ](../../argocd/README.md)を参照
- `kubectl` の現在の context が対象クラスタを指すこと
- この Canary 用設定を使う場合、`istio-system` namespace が存在すること

接続先を確認します。

```bash
kubectl config current-context
kubectl get namespace istio-system
```

## 既存 Prometheus からの移行

以前の手順で `prometheus-istio-metrics` Argo CD Application を登録している場合、先に Application と既存リソースを cascade delete します。Argo CD CLI を使う場合:

```bash
argocd app delete prometheus-istio-metrics --cascade
```

Argo CD の画面を使う場合は、`prometheus-istio-metrics` を削除するときにリソースの cascade deletion を選びます。`--cascade` はApplicationと管理対象リソースを削除します（[Argo CDの説明](https://argo-cd.readthedocs.io/en/release-3.5/user-guide/app_deletion/)）。

Argo CD CLIにログインしていない場合は、`kubectl`でApplicationをcascadeなしで削除し、旧リソースを手動で削除できます。CLIのcascade deleteがApplicationのsourceを解決できず失敗する場合も、この方法を使います。

```bash
kubectl patch application prometheus-istio-metrics -n argocd \
  --type merge -p '{"metadata":{"finalizers":null}}'
kubectl delete application prometheus-istio-metrics -n argocd
kubectl delete -n monitoring \
  deployment/prometheus service/prometheus configmap/prometheus-config \
  serviceaccount/prometheus --ignore-not-found
kubectl delete -n istio-system \
  role/prometheus-pod-discovery rolebinding/prometheus-pod-discovery \
  --ignore-not-found
```

削除完了後、旧Deployment・Service・ConfigMap・ServiceAccountと、`istio-system` namespaceのRole/RoleBindingが残っていないことを確認してからHelmfileを適用してください。

```bash
kubectl get deployment,service,configmap,serviceaccount -n monitoring
kubectl get role,rolebinding prometheus-pod-discovery -n istio-system
```

```bash
kubectl get all -n monitoring
```

この移行後、Prometheus は Argo CD ではなく Helmfile が管理します。Argo CD Application の定義 `platform/argocd/apps/prometheus-istio-metrics.yaml` は廃止しています。

## dev 設定でインストールする

リポジトリのルートで mise のツールを準備し、Prometheus のディレクトリへ移動します。

```bash
mise install
cd platform/monitoring/prometheus
```

まず差分を確認してから、インストールします。

```bash
mise exec -- helmfile -e dev diff
mise exec -- helmfile -e dev apply --interactive
```

起動状態を確認します。

```bash
kubectl get pods,services -n monitoring
```

Prometheus の画面はポートフォワードで開きます。

```bash
kubectl port-forward -n monitoring service/prometheus 9090:9090
```

ブラウザで <http://localhost:9090> を開きます。Service は ClusterIP のためクラスタ外へ直接公開されません。

## prod 設定を適用する

PVC を作成できる StorageClass があるクラスタで、次のように環境を選びます。

```bash
mise exec -- helmfile -e prod diff
mise exec -- helmfile -e prod apply --interactive
kubectl get pods,pvc -n monitoring
```

この操作も同じ Helm release を更新します。dev から prod へ切り替える場合、Pod が再作成され、永続ボリュームが有効になります。dev へ戻すと `emptyDir` の構成に戻るため、Prometheus のデータは保持されません。

## chart と値の参照先

- chart: [prometheus-community/prometheus](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus)
- chart version: `29.37.2`
- 共通設定: [`values.yaml`](values.yaml)
- dev 設定: [`environments/dev/values.yaml`](environments/dev/values.yaml)
- prod 設定: [`environments/prod/values.yaml`](environments/prod/values.yaml)

共通設定には、Istio proxy、Node Exporter、kube-state-metrics、Go アプリケーションの scrape 設定を置いています。Node Exporter は全ノードに Pod を置く DaemonSet です。現在の kind は1ノードなので、Node Exporter の scrape target も1つになります。kube-state-metrics は Kubernetes API のリソース状態を公開する Deployment です。Alertmanager と Pushgateway は無効です。環境別の values には、保存期間・ストレージ・リソース・Node CPU アラート閾値の差を置いています。

## Node Exporter の確認

Node Exporter を有効にした後、差分を確認して Helm release を更新します。

```bash
mise exec -- helmfile -e dev diff
mise exec -- helmfile -e dev apply --interactive
```

DaemonSet と配置先ノードを確認します。

```bash
kubectl get daemonset,pods -n monitoring -o wide
```

Prometheus の画面で次の PromQL を実行します。

```promql
up{job="node-exporter"}
```

`1` が表示されれば scrape できています。CPU 使用率の目安やメモリ空き容量も確認できます。

```promql
rate(node_cpu_seconds_total{mode!="idle"}[5m])
node_memory_MemAvailable_bytes
```

Node Exporter が見るのは kind の Node コンテナから見える情報です。物理ホスト全体の値と一致するとは限りません。

## Kubernetes リソース状態の確認

kube-state-metrics は Kubernetes API を読み、Pod や Deployment などの状態をメトリクスとして公開します。Node Exporter のような DaemonSet ではなく、既定では Deployment として動作します。メトリクス endpoint は `monitoring` namespace の Pod として Prometheus が検出します。kube-state-metrics 自身が Kubernetes API を読むための read-only RBAC は、サブチャートが作成します。

Helmfile の差分を確認してから dev 設定を適用します。

```bash
mise exec -- helmfile -e dev diff
mise exec -- helmfile -e dev apply --interactive
```

Pod と Deployment の起動を確認します。

```bash
kubectl get deployments,pods,services -n monitoring
```

Prometheus UI で次の PromQL を実行します。

```promql
up{job="kube-state-metrics"}
kube_deployment_status_replicas_available
```

最初の式が `1` なら scrape できています。2つ目の式は各 Deployment の利用可能なレプリカ数を返します。

## Go アプリケーションの HTTP 指標

Go アプリケーションは `/metrics` で要求数と応答時間を公開します。Prometheus は `hello-world` と `progressive-delivery` namespace の Pod のうち、`hello-world` コンテナの port `8080` を scrape します。Pod discovery 用 Role は各 namespace の Kustomize base が作成し、Prometheus ServiceAccount には Pod の get/list/watch だけを許可します。

アプリケーションの変更を `main` へ push すると、GitHub Actions がコミット SHA のイメージを GHCR へ公開します。その後、kind と progressive-delivery の両 Kustomize 設定で `newTag` をその SHA に更新し、Argo CD が新しいイメージを展開したことを確認します。Prometheus の設定は Helmfile の `dev` 環境で反映します。

```bash
mise exec -- helmfile -e dev diff
mise exec -- helmfile -e dev apply --interactive
```

Prometheus UI で scrape 状態とアプリ指標を確認します。

```promql
up{job="hello-world-app"}
hello_world_http_requests_total
rate(hello_world_http_requests_total[5m])
histogram_quantile(
  0.95,
  sum by (namespace, le) (
    rate(hello_world_http_request_duration_seconds_bucket[5m])
  )
)
```

最初の式が `1` なら scrape できています。要求数を増やすには、アプリの `/` または `/error` にリクエストを送ります。最後の式は namespace ごとの応答時間の95パーセンタイルを秒で返します。`/metrics` の scrape 自体はアプリ要求数に含めません。
