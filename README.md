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

Trivy で Dockerfile の設定を検査し、結果を `trivy-config-report` アーティファクトに保存します。検出した設定上の問題がある場合は、ビルドへ進みません。

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

## kind クラスタへのデプロイ

[`k8s/hello-world.yaml`](k8s/hello-world.yaml) は、GitHub Actions が公開したコミット SHA タグのイメージを使う Deployment と Service の例です。別のイメージを使う場合は、`image:` のタグをそのコミット SHA に置き換えてください。

```bash
kubectl apply -f k8s/hello-world.yaml
kubectl rollout status deployment/hello-world -n hello-world
kubectl port-forward -n hello-world service/hello-world 18080:8080
```

`port-forward` を実行したまま、別のターミナルからアクセスします。

```bash
curl http://localhost:18080/
```

GHCR パッケージが非公開の場合、kind クラスタからイメージを取得するために `imagePullSecret` が必要です。Actions の `GITHUB_TOKEN` はワークフローのジョブ内でのみ使えるため、ローカルのクラスタでは再利用できません。

削除する場合は次を実行します。

```bash
kubectl delete -f k8s/hello-world.yaml
```
