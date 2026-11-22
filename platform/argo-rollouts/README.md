# Argo Rolloutsのセットアップ

この手順はArgo RolloutsのコントローラーとCRDをkindクラスタへ導入します。アプリをデプロイするたびには実行しません。

## 一度だけ行うクラスタの準備

### 前提条件

- kindクラスタが起動している
- kubectlの接続先が対象クラスタである
- sec7でセットアップしたArgo CDが稼働している
- Canaryを試す場合、IstioとIngress Gatewayが稼働している。未導入の場合は次の手順で導入する

接続先を確認します。以降の操作は、このcontextが対象のkindクラスタを指している状態で行います。

    kubectl config current-context

### IstioとIngress Gatewayを用意する

このリポジトリのCanary overlayは、`istio-system` namespaceの`istio-ingressgateway` Serviceと、`istio=ingressgateway`ラベルを持つGateway Podを前提にします。Istioがすでに導入されている場合は、既存の構成を確認してください。

    kubectl get pods,services -n istio-system
    kubectl get pods -n istio-system -l istio=ingressgateway --show-labels

`istiod`と`istio-ingressgateway`が稼働していれば、再インストールは不要です。Istioが未導入で、対象クラスタへ新たに導入する場合は、以下を実行します。ここではIstio 1.31.1の`default` profileを使います。このprofileはIstioの制御プレーンとIngress Gatewayを導入します。Istio 1.31はKubernetes 1.32〜1.37をサポートするため、先にクラスタのバージョンを確認してください。

    kubectl version

リポジトリの外でIstio CLIを取得します。

    cd /tmp
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.31.1 sh -
    cd istio-1.31.1
    export PATH="$PWD/bin:$PATH"
    istioctl version

`kubectl`の接続先を再確認してから、Istioを導入します。`istioctl install`が`istio-system` namespace、制御プレーン、Ingress Gatewayを作成します。

    kubectl config current-context
    istioctl install --set profile=default -y

PodとServiceの起動を確認します。

    kubectl get pods,services -n istio-system
    kubectl get pods -n istio-system -l istio=ingressgateway --show-labels

`istio-ingressgateway` Serviceと、Gatewayのselectorに一致するPodが表示されれば準備完了です。すでにIstioを独自の方法で管理しているクラスタには、この手順を重ねて実行しないでください。Gatewayが別namespaceにある場合やPodのラベルが異なる場合は、Canary overlayのGateway selectorとIngress GatewayのService参照先をその構成に合わせます。

IstioはこのリポジトリのArgo CD Applicationでは管理しません。Canaryを使うクラスタに一度だけ導入します。バージョンを変更するときは、[Istioのサポート対象Kubernetesバージョン](https://istio.io/latest/docs/releases/supported-releases/)と[公式インストール手順](https://istio.io/latest/docs/setup/install/istioctl/)を確認してください。

Canaryのトラフィック分析にはPrometheusを使います。このリポジトリではHelm chartをHelmfileで導入し、Istioのプロキシメトリクスを収集します。`dev`と`prod`の設定と移行手順は[Prometheusのセットアップ](../monitoring/prometheus/README.md)を参照してください。すでに同じ用途のPrometheusがある場合は重ねて導入せず、Canary overlayのAnalysisTemplateにある接続先を既存のService名へ変更してください。

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

### Canary用Prometheusをインストールする

Canary用PrometheusをHelmfileでインストールします。既存のArgo CD管理から移行する場合を含め、詳しい手順は[Prometheusのセットアップ](../monitoring/prometheus/README.md)を参照してください。PrometheusはIstio proxy、Node Exporter、kube-state-metricsに加え、`hello-world`と`progressive-delivery` namespaceのGoアプリからもメトリクスを収集します。Pod discovery のRoleBindingは各Kustomize baseが作成します。

BlueGreenだけを試す場合、Prometheusは必要ありません。IstioもBlueGreen自体には必要ありませんが、このリポジトリのCanary overlayではIstioをトラフィックルーターとして使います。

## アプリケーションの登録

プログレッシブデリバリ用のArgo CD Applicationは[k8s/progressive-delivery/README.md](../../k8s/progressive-delivery/README.md)の手順で一度登録します。以後のアプリ更新はGit上のimage tag変更で行います。

## バージョンの参照元

- [Argo Rollouts Helm chart](https://github.com/argoproj/argo-helm/tree/main/charts/argo-rollouts)
- [Argo Rolloutsのインストール](https://argoproj.github.io/argo-rollouts/installation/)
