import * as lambda from 'aws-cdk-lib/aws-lambda'
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

// goLambdaCode builds a Go Lambda binary from moduleDir. cmdPath is the package
// to build, relative to moduleDir ('./cmd/worker' for the worker, '.' for the egress).
// Local bundling (no Docker) is attempted first; Docker is the fallback.
export function goLambdaCode(moduleDir: string, cmdPath: string): lambda.AssetCode {
  return lambda.Code.fromAsset(moduleDir, {
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
