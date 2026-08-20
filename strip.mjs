import { readdirSync, readFileSync, writeFileSync } from 'fs';
import { join, extname } from 'path';

const dir = process.argv[2] || '.';

function stripGo(src) {
  const out = [];
  let i = 0;
  const n = src.length;

  while (i < n) {
    // String literal — copy verbatim, no comment stripping inside
    if (src[i] === '"') {
      let s = '"';
      i++;
      while (i < n && src[i] !== '"') {
        if (src[i] === '\\') { s += src[i++]; }
        s += src[i++];
      }
      s += '"';
      i++;
      out.push(s);
      continue;
    }

    // Raw string literal — copy verbatim
    if (src[i] === '`') {
      let s = '`';
      i++;
      while (i < n && src[i] !== '`') { s += src[i++]; }
      s += '`';
      i++;
      out.push(s);
      continue;
    }

    // Rune literal — copy verbatim
    if (src[i] === "'") {
      let s = "'";
      i++;
      while (i < n && src[i] !== "'") {
        if (src[i] === '\\') { s += src[i++]; }
        s += src[i++];
      }
      s += "'";
      i++;
      out.push(s);
      continue;
    }

    // Single-line comment — skip to end of line, preserve the newline
    if (src[i] === '/' && src[i + 1] === '/') {
      while (i < n && src[i] !== '\n') i++;
      continue;
    }

    // Multi-line comment — skip everything including newlines
    if (src[i] === '/' && src[i + 1] === '*') {
      i += 2;
      while (i < n && !(src[i] === '*' && src[i + 1] === '/')) i++;
      i += 2;
      continue;
    }

    out.push(src[i++]);
  }

  return out.join('');
}

function cleanLines(src) {
  return src
    .split('\n')
    .map(l => l.trimEnd())   // trim trailing whitespace
    .filter(l => l.trim() !== '') // remove blank / whitespace-only lines
    .join('\n') + '\n';
}

const files = readdirSync(dir).filter(f => extname(f) === '.go');

if (files.length === 0) {
  console.log('No .go files found in', dir);
  process.exit(0);
}

for (const file of files) {
  const path = join(dir, file);
  const original = readFileSync(path, 'utf8');
  const stripped = cleanLines(stripGo(original));
  writeFileSync(path, stripped, 'utf8');
  const before = original.split('\n').length;
  const after  = stripped.split('\n').length;
  console.log(`  ${file.padEnd(20)} ${before} → ${after} lines  (-${before - after})`);
}

console.log(`\nDone. Processed ${files.length} file(s).`);
