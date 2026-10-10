# 7.2.3.2 Argo CD のセットアップ

このディレクトリでは、Helmfile を使って既存の Kubernetes クラスタへ Argo CD をインストールします。Argo CD 自体の起動までを扱います。アプリケーションの GitOps 管理は、リポジトリ直下の README にある「7.2.4 Argo CD による GitOps デプロイ」で設定します。

アプリケーションのビルドや GHCR への push を行う CI とは役割が異なるため、設定を `platform/argocd/` に分けています。

## 使用するバージョン

Helm と Helmfile のバージョンは、リポジトリ直下の [`mise.toml`](../../mise.toml) で固定します。Helm の差分表示に使う helm-diff plugin は `3.15.15`、Argo CD Helm chart は `10.10.2` です。この chart がインストールする Argo CD は `3.5.4` です。chart のソースは [argoproj/argo-helm](https://github.com/argoproj/argo-helm/tree/main/charts/argo-cd) で確認できます。

## 前提条件

- mise がインストールされていること
- kind クラスタが起動していること
- `kubectl` の現在の context が、インストール先の kind クラスタを指していること

まず接続先を確認します。

```bash
kubectl config current-context
kubectl cluster-info
```

Helmfile の `dev` と `prod` は values ファイルを選ぶ名前です。接続先クラスタは `kubectl` の現在の context で決まります。この手順では kind クラスタを接続先にし、`dev` を選択します。

## インストール

リポジトリのルートで、mise が管理するツールをインストールします。

```bash
mise install
```

続けて、このディレクトリへ移動し、Helm 4 の署名検証に対応した helm-diff のリリースアーカイブをインストールします。前回の手順で `HELM_PLUGINS` を設定している場合は、標準の plugin ディレクトリを使うため解除します。すでに `helm diff version` が `3.15.15` を表示する場合、インストールは不要です。

```bash
cd platform/argocd
unset HELM_PLUGINS
curl -fsSL https://github.com/databus23.gpg | gpg --import
gpg --list-keys --with-fingerprint EA17A2A206AFF8CD
mise exec -- helm plugin install https://github.com/databus23/helm-diff/releases/download/v3.15.15/helm-diff-linux-amd64.tgz
```

表示された fingerprint が `C5645EF4 7482257A 1F806D2B EA17A2A2 06AFF8CD` と一致することを確認してください。上記アーカイブは Linux amd64 用です。他の OS や CPU アーキテクチャでは、対応するアーカイブを選びます。helm-diff は Helm の plugin としてインストールします。`HELM_PLUGINS` を mise の helm-diff 配布ディレクトリに設定すると、Helm の標準 plugin ディレクトリが検索対象から外れ、他の plugin を見つけられなくなるため設定しません。

インストールできたことを確認します。

```bash
mise exec -- helm diff version
```

この Helmfile では secrets ファイルを使わないため、`helmfile init` は実行しません。`init` は helm-diff 以外に helm-secrets なども検査・導入しようとします。Helmfile の `diff` と `apply` に必要な helm-diff は、上記の手順で個別に導入しています。

まず、適用する変更内容を確認します。

```bash
mise exec -- helmfile -e dev diff
```

初回は Argo CD のリソースがまとめて表示されます。内容を確認してからインストールします。

```bash
mise exec -- helmfile -e dev apply --interactive
```

Helmfile は `diff` でクラスタとの差分を表示し、`--interactive` を指定すると確認後にインストールします。`atomic: true` により、インストールに失敗した場合は Helm がそのリリースをロールバックします。

Pod の起動を確認します。

```bash
kubectl get pods -n argocd
```

## Argo CD の画面を開く

`dev` では Argo CD server の HTTP を有効にしています。ポートフォワードはローカルからのみ使い、Service を外部へ公開しないでください。

```bash
kubectl port-forward -n argocd service/argocd-server 8080:80
```

ブラウザで <http://localhost:8080> を開きます。初期ユーザー名は `admin` です。初期パスワードは次のコマンドで確認できます。

```bash
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath='{.data.password}' | base64 --decode
printf '\n'
```

初期パスワードをソースコードへ保存しないでください。ログイン後に初期パスワードを変更したら、初期パスワードを保持する Secret を削除します。

```bash
kubectl delete secret argocd-initial-admin-secret -n argocd
```

長期間使う環境では、TLS と認証も適切に設定してください。

## 環境別の設定

- `environments/dev/values.yaml` は、kind で画面を確認するため `server.insecure` を有効にします。HTTP を使うため、この設定はローカルのポートフォワードに限ってください。
- `environments/prod/values.yaml` は `server.insecure` を無効にし、Argo CD server の TLS を使います。

どちらの values ファイルも、chart の全デフォルト値を複製せず、環境ごとに変更する値だけを記述しています。
