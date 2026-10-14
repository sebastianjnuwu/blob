const { SignatureV4 } = require('@smithy/signature-v4');
const crypto = require('node:crypto');

class Sha256 {
  constructor(secret) { this.h = secret ? crypto.createHmac('sha256', Buffer.from(secret)) : crypto.createHash('sha256'); }
  update(d) { this.h.update(Buffer.isBuffer(d) ? d : Buffer.from(d)); }
  digest() { return this.h.digest(); }
}

(async () => {
  const signer = new SignatureV4({
    credentials: { accessKeyId: 'TESTKEY1234567890AB', secretAccessKey: 'abcdefghijklmnopqrstuvwxyz0123456789ABCD' },
    region: 'us-east-1', service: 's3', sha256: Sha256,
  });
  const req = {
    protocol: 'http:', hostname: '127.0.0.1', port: 3099, method: 'GET', path: '/blob/abc',
    query: { 'list-type': '2', prefix: 'a b' },
    headers: { host: '127.0.0.1:3099' },
  };
  const signed = await signer.sign(req, { signingDate: new Date('2025-01-01T00:00:00Z') });
  console.log(JSON.stringify({ auth: signed.headers['authorization'], date: signed.headers['x-amz-date'], sha: signed.headers['x-amz-content-sha256'] }));
})().catch((e) => { console.error('ERR', e.message); process.exit(1); });
