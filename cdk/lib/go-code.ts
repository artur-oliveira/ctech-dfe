import * as cdk from 'aws-cdk-lib'
import * as lambda from 'aws-cdk-lib/aws-lambda'
import {createHash} from 'node:crypto'
import fs from 'node:fs'
import {spawnSync} from 'child_process'
import path from 'node:path'

// resolveGo returns the absolute path to the go binary.
// Checks PATH first, then falls back to ~/sdk/go*/bin/go (Google's default SDK dir).
function resolveGo(): string {
  const lookup = spawnSync('bash', ['-c',
    'which go 2>/dev/null || ls "${HOME}/sdk/go"*/bin/go 2>/dev/null | sort -rV | head -1',
  ], {stdio: 'pipe', env: process.env})
  if (lookup.status === 0 && lookup.stdout) {
    const found = lookup.stdout.toString().trim()
    if (found) return found
  }
  return 'go'
}

// Código-fonte que todo Lambda Go deste repo compila além do próprio módulo: go-dfe entra por `replace` no go.mod
// (api, worker e egress) e o go.work/go.work.sum fixam as versões. O asset do CDK só fingerprinta moduleDir; sem
// este hash, uma mudança só em go-dfe não muda o asset e o Lambda não é atualizado (foi como o conserto do tpOper
// do NFS-e ficou fora do egress).
const REPO_ROOT = path.join(__dirname, '../..')
const SHARED_SOURCES = [path.join(REPO_ROOT, 'go-dfe'), path.join(REPO_ROOT, 'go.work'), path.join(REPO_ROOT, 'go.work.sum')]

// goSourceHash junta o fingerprint do módulo e o das fontes compartilhadas (ignora o que não vira binário).
export function goSourceHash(moduleDir: string, extra: string[] = SHARED_SOURCES): string {
  const h = createHash('sha256')
  for (const p of [moduleDir, ...extra]) {
    if (!fs.existsSync(p)) continue
    h.update(p === moduleDir ? 'module' : path.basename(p))
    h.update(cdk.FileSystem.fingerprint(p, {exclude: ['*_test.go', 'testdata', '.git', 'node_modules']}))
  }
  return h.digest('hex')
}

// goLambdaCode builds a Go Lambda binary from moduleDir. cmdPath is the package
// to build, relative to moduleDir ('./cmd/worker' for the worker, '.' for the egress).
// Local bundling (no Docker) is attempted first; Docker is the fallback.
export function goLambdaCode(moduleDir: string, cmdPath: string): lambda.AssetCode {
  return lambda.Code.fromAsset(moduleDir, {
    assetHashType: cdk.AssetHashType.CUSTOM,
    assetHash: goSourceHash(moduleDir),
    bundling: {
      local: {
        tryBundle(outputDir: string): boolean {
          const r = spawnSync(
            resolveGo(),
            ['build', '-tags', 'lambda.norpc', '-ldflags', '-s -w', '-o', path.join(outputDir, 'bootstrap'), cmdPath],
            {
              cwd: moduleDir,
              env: {...process.env, GOOS: 'linux', GOARCH: 'arm64', CGO_ENABLED: '0'},
              stdio: ['ignore', 'pipe', 'pipe'],
            },
          )
          if (r.status !== 0) process.stderr.write(r.stderr ?? Buffer.alloc(0))
          return r.status === 0
        },
      },
      image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
      // GOCACHE/GOPATH must be writable; Docker runs as uid 1000:1000 which has no HOME.
      environment: {GOCACHE: '/tmp/go-build', GOPATH: '/tmp/go'},
      command: [
        'bash', '-c',
        `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o /asset-output/bootstrap ${cmdPath}`,
      ],
    },
  })
}
