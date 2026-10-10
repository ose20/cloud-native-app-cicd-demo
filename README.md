# cloud-native-app-cicd-demo

『クラウドネイティブで実現するマイクロサービス開発・運用実践ガイド』第7章のサンプルを、GitHub 向けに再現するリポジトリです。

https://gihyo.jp/book/2023/978-4-297-13783-0

## 7.1.2.2 コンテナイメージビルド＆プッシュ

Go の HTTP サーバーをコンテナイメージにビルドし、GitHub Container Registry（GHCR）へ公開します。Go のバージョンは [`mise.toml`](mise.toml) で管理します。

### ローカルでの実行

```bash
mise install
mise exec -- go test -v ./...
mise exec -- go run .
```

`http://localhost:8080/` にアクセスすると `hello world` を返します。

### コンテナイメージのビルドと実行

```bash
docker build -t hello-world:local .
docker run --rm -p 8080:8080 --name hello-world hello-world:local
```

### GitHub Actions

- Pull Request ではコンテナイメージをビルドします。
- `main` ブランチへの push では、コミット SHA をタグにして `ghcr.io/<owner>/<repository>` へビルド・プッシュします。
- GHCR への認証には `GITHUB_TOKEN` を使います。ワークフローに `packages: write` 権限を設定しています。
