# Argo Rolloutsのセットアップ

この手順はArgo RolloutsのコントローラーとCRDをkindクラスタへ導入します。アプリをデプロイするたびには実行しません。

## 一度だけ行うクラスタの準備

### 前提条件

- kindクラスタが起動している
- kubectlの接続先が対象クラスタである
- sec7でセットアップしたArgo CDが稼働している
- Canaryを試す場合、IstioとIngress Gatewayが稼働している

接続先とIstioを確認します。

    kubectl config current-context
    kubectl get pods -n istio-system
    kubectl get service istio-ingressgateway -n istio-system

Canaryのトラフィック分析にはPrometheusも使います。このリポジトリにはIstioのプロキシメトリクスを収集するローカル学習用のPrometheus設定を置いています。データはPodの再起動で消える一時領域に保存し、24時間で削除します。すでに同じ用途のPrometheusがある場合は重ねて導入せず、Canary overlayのAnalysisTemplateにある接続先を既存のService名へ変更してください。

Istioのsidecar injectionにrevision labelを使っているクラスタでは、progressive-delivery namespaceとRollout templateのinjection設定を、そのrevisionに合わせてください。

### miseでローカルツールを用意する

Argo Rolloutsのkubectlプラグインをmiseで管理しています。リポジトリのルートで一度インストールします。

    mise install
    mise exec -- kubectl argo rollouts version

Helmfileが使うhelm-diff pluginはArgo CDのセットアップで導入済みであることを前提にします。まだの場合は[Argo CDのセットアップ手順](../argocd/README.md)を先に行ってください。

### コントローラーとCRDをインストールする

Helm chartのバージョンは2.43.6、コントローラーとkubectlプラグインのバージョンは1.10.0に固定しています。chartの設定はこのディレクトリにあります。

    cd platform/argo-rollouts
    mise exec -- helmfile diff
    mise exec -- helmfile apply --interactive

差分を確認してから初回インストールを承認してください。chartは専用namespaceのargo-rolloutsにコントローラーとCRDを作成します。values.yamlではkind向けにコントローラーを1 replicaにしています。

導入後に確認します。

    kubectl get pods -n argo-rollouts
    kubectl get crd rollouts.argoproj.io analysistemplates.argoproj.io analysisruns.argoproj.io

このHelmfileに対するapplyは、バージョン更新や設定変更をするときにも使います。通常のアプリリリースでは実行しません。

### Canary用Prometheusを登録する

Prometheusを新たに導入する場合だけ、Argo CD Applicationを一度登録します。

    cd ../..
    kubectl apply -f platform/argocd/apps/prometheus-istio-metrics.yaml
    kubectl get application prometheus-istio-metrics -n argocd
    kubectl get pods -n monitoring

Argo CDがplatform/monitoring/prometheusを同期します。Prometheusはistio-systemとprogressive-delivery namespaceのIstio proxyからメトリクスを収集します。CanaryのRoleBindingはCanary overlayが作成します。

BlueGreenだけを試す場合、Prometheusは必要ありません。IstioもBlueGreen自体には必要ありませんが、このリポジトリのCanary overlayではIstioをトラフィックルーターとして使います。

## アプリケーションの登録

プログレッシブデリバリ用のArgo CD Applicationは[k8s/progressive-delivery/README.md](../../k8s/progressive-delivery/README.md)の手順で一度登録します。以後のアプリ更新はGit上のimage tag変更で行います。

## バージョンの参照元

- [Argo Rollouts Helm chart](https://github.com/argoproj/argo-helm/tree/main/charts/argo-rollouts)
- [Argo Rolloutsのインストール](https://argoproj.github.io/argo-rollouts/installation/)
