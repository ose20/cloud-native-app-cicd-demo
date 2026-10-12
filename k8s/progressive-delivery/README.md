# Argo Rolloutsによるプログレッシブデリバリ

このディレクトリでは、同じGoアプリをCanaryとBlueGreenの両方でデプロイできます。Argo CD Applicationは一つで、参照するoverlayを選ぶことで方式を切り替えます。sec7のDeploymentはhello-world namespaceに残し、この演習はprogressive-delivery namespaceで動かします。

## 方式の違い

|方式 | トラフィックと判定 |
|---|---|
|Canary | IstioでIngressとクラスタ内通信の両方を新バージョンへ20%、50%、80%、100%と振り分けます。Prometheusの成功率が95%以上かを30秒ごとに測り、失敗測定が3回になるとRolloutを中止します。 |
|BlueGreen | 旧バージョンをactiveServiceで提供したまま、新バージョンをpreviewServiceで起動します。イメージ更新時にKubernetes Jobがpreviewへアクセスし、/が200、/errorが503であることを確認します。Job成功後に自動で切り替えます。 |

Argo Rolloutsのコントローラーは両方式を扱いますが、一つのRolloutで有効にする方式は一つです。Canaryは実トラフィックのメトリクスを観察し、BlueGreenは切替前に指定したテストを実行します。

初回のRollout作成には切り替え元のバージョンがありません。Canaryの段階的なトラフィック移行とBlueGreenの切替前Analysisは、次回以降のイメージ更新で確認できます。

アプリは/でビルドしたコミットSHAを返し、/errorで意図的に503を返します。BlueGreenのJobはこの二つの応答を確認します。E2Eテストを増やす場合は、[overlays/bluegreen/analysis-template.yaml](overlays/bluegreen/analysis-template.yaml)のJobをテストランナーに置き換えてください。

## 一度だけ行うセットアップ

初回セットアップはクラスタ単位で行います。

1. [Argo RolloutsのコントローラーとCRD](../../platform/argo-rollouts/README.md)をHelmfileでインストールする。
2. Canaryを使う場合、IstioとPrometheusを用意する。
3. このリポジトリの変更をmainへpushし、GitHub Actionsが/errorを含む新しいイメージをGHCRへ公開したことを確認する。
4. base/kustomization.yamlのnewTagを、そのイメージをビルドしたコミットSHAへ変更してpushする。
5. このArgo CD Applicationを一度登録する。

       kubectl apply -f platform/argocd/apps/progressive-delivery.yaml
       kubectl get application progressive-delivery -n argocd

Applicationの初期設定はCanaryです。Argo CDは[overlays/canary](overlays/canary)を監視します。イメージが非公開の場合は、sec7と同じくkindクラスタにGHCR用imagePullSecretを設定してください。

Application登録後、Argo CDがRollout、Services、Istioリソースを同期します。通常のリリース時にApplicationを再登録する必要はありません。

## リリースのたびに行う作業

各リリースでは、CIが公開したイメージのSHA tagをGitに指定します。

1. アプリケーションの変更をpushし、GitHub Actionsのpublish jobが成功したことを確認する。
2. [base/kustomization.yaml](base/kustomization.yaml)のnewTagを、公開されたコミットSHAへ変更する。
3. 変更をpushする。Argo CDがRolloutを更新し、選択中の方式でデプロイする。

Canaryを使うときは、Prometheusが判定できるようリクエストを継続して送ります。最初のRollout作成時には旧バージョンとの比較がないため、Canaryの段階的更新は、二つ目以降のイメージ更新で確認できます。

Canaryのトラフィックルーティングは、Ingress Gateway用の[VirtualService](overlays/canary/virtual-service.yaml)と、メッシュ内通信用の[VirtualService](overlays/canary/mesh-virtual-service.yaml)に分けています。Argo Rolloutsは両方のweightを同時に更新します。クラスタ内の呼び出し元はstable ServiceのFQDNを使い、Istioのサイドカーを経由してアクセスします。Canary Serviceを呼び出し元に直接指定する必要はありません。

まず別のターミナルでIstio Ingress Gatewayをローカルへ転送します。

    kubectl port-forward -n istio-system service/istio-ingressgateway 18080:80

もう一つのターミナルから、正常リクエストを繰り返します。

    while true; do
      curl --silent --show-error --include -H 'Host: hello-world.local' http://localhost:18080/
      sleep 0.2
    done

Rolloutの状態とAnalysisRunを確認します。

    kubectl argo rollouts get rollout hello-world -n progressive-delivery --watch
    kubectl get analysisruns -n progressive-delivery
    kubectl get virtualservice hello-world -n progressive-delivery -o yaml
    kubectl get virtualservice hello-world-mesh -n progressive-delivery -o yaml

Argo CDの画面から見る場合は、Applicationのリソースツリーで`hello-world`のRolloutを開き、Rollout Extensionのタブを選びます。Extensionの導入方法は[Argo CDのセットアップ](../../platform/argocd/README.md)を参照してください。

クラスタ内の呼び出し元PodからCanaryの振り分けを確認する場合は、progressive-delivery namespaceに一時的なcurl Podを作ります。namespaceのsidecar injectionが有効なため、このPodもメッシュに参加します。

    kubectl run internal-caller -n progressive-delivery \
      --image=curlimages/curl:8.21.0 \
      --restart=Never --command -- sleep 3600
    kubectl wait --for=condition=Ready pod/internal-caller \
      -n progressive-delivery --timeout=120s

Canaryが20%の段階にいる間に、stable ServiceのFQDNへ100回アクセスします。応答のバージョンごとの件数を確認できます。

    kubectl exec -n progressive-delivery internal-caller -- sh -c '
    i=0
    while [ "$i" -lt 100 ]; do
      curl --fail --silent --show-error \
        http://hello-world-stable.progressive-delivery.svc.cluster.local:8080/
      printf "\n"
      i=$((i + 1))
    done
    ' | sort | uniq -c

Canaryの応答が約20%含まれていれば、内部呼び出しもVirtualServiceの重み付けを通っています。100回では割合にばらつきが出ます。確認後は一時Podを削除します。

    kubectl delete pod internal-caller -n progressive-delivery

/errorへリクエストすると、カナリア側の成功率を下げて自動中止を確認できます。正常リクエストの送信を止めるとメトリクス不足で失敗判定になる場合があるため、検証中はリクエストを継続してください。

次のコマンドは/errorへ100回アクセスし、カナリア側へ十分な失敗レスポンスを送ります。

    i=0
    while [ "$i" -lt 100 ]; do
      curl --silent --output /dev/null -H 'Host: hello-world.local' http://localhost:18080/error
      i=$((i + 1))
    done

BlueGreenでは同じimage tagの更新をpushすると、切替前にJobが自動実行されます。結果は次で確認できます。

    kubectl get analysisruns,jobs -n progressive-delivery
    kubectl argo rollouts get rollout hello-world -n progressive-delivery --watch

## CanaryとBlueGreenを切り替えるとき

現在のRolloutが完了してHealthyになってから方式を切り替えます。移行中に切り替えると、旧方式のServiceやルートがArgo CDのprune対象になります。

1. [platform/argocd/apps/progressive-delivery.yaml](../../platform/argocd/apps/progressive-delivery.yaml)のspec.source.pathを変更する。

   BlueGreenを使う場合:

       path: k8s/progressive-delivery/overlays/bluegreen

   Canaryに戻す場合:

       path: k8s/progressive-delivery/overlays/canary

2. Applicationの変更をクラスタへ反映する。

       kubectl apply -f platform/argocd/apps/progressive-delivery.yaml

この切替操作は方式を変えるときだけ行います。各リリースで変更するのはimage tagです。

Argo RolloutsはCanaryのIstio VirtualService weightと、各方式のService selectorを実行中に変更します。Argo CD Applicationではこれらの差分を無視し、Rolloutsの進行をGitの初期値へ戻さないようにしています。
