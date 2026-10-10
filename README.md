# cloud-native-app-cicd-demo

『クラウドネイティブで実現するマイクロサービス開発・運用実践ガイド』第7章のサンプルを、GitHub 向けに再現するリポジトリです。

https://gihyo.jp/book/2023/978-4-297-13783-0

## 7.1.2.2 コンテナイメージビルド＆プッシュ

Go の HTTP サーバーをコンテナイメージにビルドし、GitHub Container Registry（GHCR）へ公開します。Go と CI で使うツールのバージョンは [`mise.toml`](mise.toml) で管理します。

### ローカルでの実行

```bash
mise install
mise exec -- go test -v ./...
mise exec -- go run .
```

`http://localhost:8080/` にアクセスすると `hello world` を返します。

### コンテナイメージのビルドと実行

```bash
GO_VERSION="$(mise exec -- go env GOVERSION)"
docker build --build-arg "GO_VERSION=${GO_VERSION#go}" -t hello-world:local .
docker run --rm -p 8080:8080 --name hello-world hello-world:local
```

Dockerfile のビルド用 Go イメージも、`mise.toml` から取得したバージョンを使います。

## 7.1.2.3 自動テスト

Go のテストを実行し、JUnit 形式のテスト結果と Cobertura 形式のカバレッジレポートを作成します。CI の実行結果から `test-and-coverage-reports` アーティファクトをダウンロードできます。

ローカルでレポートを作成する場合は、次を実行します。

```bash
mise exec -- gotestsum --junitfile junit-report.xml -- -coverprofile=coverprofile.txt ./...
mise exec -- gocover-cobertura < coverprofile.txt > coverprofile.xml
mise exec -- go tool cover -func=coverprofile.txt
```

## 7.1.2.4 ソースコード解析

解析ルールは [`.golangci.yaml`](.golangci.yaml) で定義します。ローカルで静的解析を実行するコマンドは次のとおりです。

```bash
mise exec -- golangci-lint run ./...
```

## 7.1.2.5 Dockerfile スキャン

Trivy で Dockerfile の設定を検査し、結果を `trivy-config-report` アーティファクトに保存します。LOW の指摘はレポートに残し、MEDIUM 以上の指摘がある場合はビルドへ進みません。

ローカルで確認する場合は次を実行します。

```bash
mise exec -- trivy config Dockerfile
```

## 7.1.2.6 コンテナイメージスキャン

CI では、ビルドしたイメージを `image.tar` として次のジョブへ渡し、Trivy で脆弱性を検査します。CRITICAL の脆弱性が見つかった場合は GHCR へ push しません。スキャン結果は `trivy-image-report` アーティファクトから確認できます。

ローカルで確認する場合は、イメージをビルドしてから次を実行します。

```bash
mise exec -- trivy image --scanners vuln hello-world:local
```

### GitHub Actions

- Pull Request と `main` ブランチへの push で、Dockerfile スキャン、lint、test、イメージのビルドと脆弱性スキャンを実行します。
- `main` ブランチへの push では、スキャンに成功したイメージをコミット SHA タグで `ghcr.io/<owner>/<repository>` へ push します。
- `v` で始まる Git タグの push では、既に GHCR にあるコミット SHA タグのイメージを使い、Git タグ名と `latest` のタグを付けて push します。この場合、イメージのビルドとスキャンは実行しません。
- `mise-action` が [`mise.toml`](mise.toml) の Go バージョンを使い、Dockerfile のビルド用 Go バージョンにも渡します。
- Trivy のバージョンも [`mise.toml`](mise.toml) で管理します。
- GHCR への認証には `GITHUB_TOKEN` を使います。ワークフローに `packages: write` 権限を設定しています。

## 7.1.2.7 本番環境へリリース可能なイメージタグを作成

`main` ブランチへの push で公開した SHA タグ付きイメージに、Git のバージョンタグと `latest` を付けます。タグを付けるコミットは、`main` の CI が成功し、SHA タグのイメージが GHCR に公開済みである必要があります。

例えば、`main` の最新コミットに `v1.0.0` タグを付けて push します。

```bash
git switch main
git pull --ff-only
git tag v1.0.0
git push origin v1.0.0
```

タグの push で `tag-image` ジョブが実行され、同じイメージを次のタグ名で GHCR に公開します。

```text
ghcr.io/<owner>/<repository>:v1.0.0
ghcr.io/<owner>/<repository>:latest
```

`latest` は最後に実行されたタグのイメージを指します。

## Kubernetes マニフェスト

アプリケーションの Kubernetes マニフェストは Kustomize で管理します。[`k8s/base/`](k8s/base/) に Namespace・Deployment・Service を置き、[`k8s/overlays/kind/`](k8s/overlays/kind/) で kind クラスタ用のイメージタグを指定しています。

適用前に、Kustomize が生成するマニフェストを確認できます。

```bash
kubectl kustomize k8s/overlays/kind
```

この overlay を kind クラスタへ適用する方法は、次の 7.2.4 で説明します。Argo CD に登録した後は Argo CD が状態を管理するため、`kubectl apply` による変更を重ねないでください。

## 7.2.3.2 Argo CD のセットアップ

既存の Kubernetes クラスタへ Argo CD を Helmfile でインストールします。手順と設定ファイルは [`platform/argocd/README.md`](platform/argocd/README.md) にあります。この設定は Argo CD のセットアップ用で、アプリケーションの CI ワークフローには含めていません。

## 7.2.4 Argo CD による GitOps デプロイ

7.2.3.2 でインストールした Argo CD に、このリポジトリのアプリケーションを登録します。Argo CD は `main` ブランチの [`k8s/overlays/kind/`](k8s/overlays/kind/) を監視し、Namespace・Deployment・Service を kind クラスタへ同期します。

### 前提条件

- Argo CD が kind クラスタで起動していること
- `kubectl` の context がその kind クラスタを指していること
- Argo CD から GitHub リポジトリを読み取れること
- kind クラスタから GHCR のイメージを取得できること

公開リポジトリであれば GitHub の認証情報は不要です。非公開リポジトリの場合は、読み取り用の認証情報を Argo CD に登録してください。認証情報を YAML や Git に保存しないでください。

GHCR パッケージが非公開の場合は、GitHub のリポジトリ認証とは別に、kind クラスタが使う `imagePullSecret` が必要です。Actions の `GITHUB_TOKEN` はワークフローのジョブ内だけで使えるため、クラスタでは再利用できません。パッケージが公開されている場合、この設定は不要です。

### Application の登録

[`platform/argocd/apps/hello-world.yaml`](platform/argocd/apps/hello-world.yaml) が Argo CD の Application 定義です。Argo CD をインストールした後、次を一度実行します。

Application 自体は Argo CD の監視対象パスの外にあるため、この `kubectl apply` が初回登録の操作です。以後、Application が監視するアプリケーションの Kubernetes マニフェストは Git の push に応じて同期されます。

```bash
kubectl apply -f platform/argocd/apps/hello-world.yaml
kubectl get application hello-world -n argocd
kubectl get pods -n hello-world --watch
```

Application の自動同期が有効なため、Argo CD は Git のマニフェストとクラスタの状態を合わせます。`selfHeal` はクラスタ上で直接変更したリソースを Git の内容へ戻し、`prune` は Git から削除されたリソースをクラスタからも削除します。

Pod が起動したら、Service にポートフォワードしてアプリケーションを確認します。

```bash
kubectl port-forward -n hello-world service/hello-world 18080:8080
```

別のターミナルからアクセスします。

```bash
curl http://localhost:18080/
```

### イメージを更新する

GitHub Actions は `main` への push ごとにコミット SHA タグのイメージを GHCR へ公開します。CI の成功後、[`k8s/overlays/kind/kustomization.yaml`](k8s/overlays/kind/kustomization.yaml) の `newTag` を、公開済みイメージのコミット SHA に変更して push します。Argo CD が Git の変更を検出し、新しいイメージへ Deployment を更新します。

CI はイメージを公開しますが、Kustomize のタグは自動更新しません。どのイメージをデプロイするかも Git の変更として管理するためです。
