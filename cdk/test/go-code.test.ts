import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {goSourceHash} from '../lib/go-code'

describe('goSourceHash', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gocode-'))
  const mod = path.join(tmp, 'mod')
  const shared = path.join(tmp, 'go-dfe')
  fs.mkdirSync(mod)
  fs.mkdirSync(shared)
  fs.writeFileSync(path.join(mod, 'main.go'), 'package main')
  fs.writeFileSync(path.join(shared, 'a.go'), 'package a')

  it('muda quando só a fonte compartilhada (go-dfe) muda', () => {
    const before = goSourceHash(mod, [shared])
    fs.writeFileSync(path.join(shared, 'a.go'), 'package a // fix')
    expect(goSourceHash(mod, [shared])).not.toBe(before)
  })

  it('não muda por arquivo de teste', () => {
    const before = goSourceHash(mod, [shared])
    fs.writeFileSync(path.join(shared, 'a_test.go'), 'package a')
    expect(goSourceHash(mod, [shared])).toBe(before)
  })
})
