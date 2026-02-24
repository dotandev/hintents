import { KmsEd25519Signer } from '../src/audit/signing/kmsSigner';
import { AuditLogger } from '../src/audit/AuditLogger';
import { verifyAuditLog } from '../src/audit/AuditVerifier';
import { KMSClient, SignCommand } from '@aws-sdk/client-kms';

jest.mock('@aws-sdk/client-kms');

describe('KMS audit signing', () => {
  const mockKeyId = 'arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012';
  const mockPublicKeyPem = process.env.TEST_PUBLIC_KEY_PEM || '';

  const mockSignature = Buffer.from('signature_bytes_from_kms_api');

  beforeEach(() => {
    process.env.ERST_KMS_REGION = 'us-east-1';
    process.env.ERST_KMS_KEY_ID = mockKeyId;
    process.env.ERST_KMS_PUBLIC_KEY_PEM = mockPublicKeyPem;

    jest.clearAllMocks();

    const mockSend = jest.fn().mockResolvedValue({
      Signature: mockSignature,
    });

    (KMSClient as jest.MockedClass<typeof KMSClient>).mockImplementation(
      () => ({
        send: mockSend,
      } as any)
    );
  });

  afterEach(() => {
    delete process.env.ERST_KMS_REGION;
    delete process.env.ERST_KMS_KEY_ID;
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
  });

  test('throws when ERST_KMS_KEY_ID is missing', () => {
    delete process.env.ERST_KMS_KEY_ID;
    expect(() => new KmsEd25519Signer()).toThrow(
      'KMS signer selected but ERST_KMS_KEY_ID is not set'
    );
  });

  test('throws when ERST_KMS_PUBLIC_KEY_PEM is missing', () => {
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
    expect(() => new KmsEd25519Signer()).toThrow(
      'KMS signer selected but ERST_KMS_PUBLIC_KEY_PEM is not set'
    );
  });

  test('initializes with correct region from env or default', () => {
    const signer = new KmsEd25519Signer();
    expect(signer).toBeDefined();
    expect(KMSClient).toHaveBeenCalledWith({ region: 'us-east-1' });
  });

  test('initializes with custom region from env', () => {
    process.env.ERST_KMS_REGION = 'us-west-2';
    new KmsEd25519Signer();
    expect(KMSClient).toHaveBeenCalledWith({ region: 'us-west-2' });
  });

  test('invokes KMS SignCommand with Ed25519 algorithm', async () => {
    const signer = new KmsEd25519Signer();
    const payload = Buffer.from('test payload');

    await signer.sign(payload);

    const mockInstance = (KMSClient as jest.MockedClass<typeof KMSClient>).mock
      .results[0].value as any;
    expect(mockInstance.send).toHaveBeenCalledWith(expect.any(SignCommand));

    const callArg = mockInstance.send.mock.calls[0][0] as SignCommand;
    expect(callArg.input.KeyId).toBe(mockKeyId);
    expect(callArg.input.Message).toEqual(payload);
    expect(callArg.input.SigningAlgorithm).toBe('Ed25519');
    expect(callArg.input.MessageFormat).toBe('RAW');
  });

  test('returns signature from KMS API response', async () => {
    const signer = new KmsEd25519Signer();
    const payload = Buffer.from('test payload');

    const signature = await signer.sign(payload);

    expect(signature).toEqual(mockSignature);
  });

  test('returns public key from environment', async () => {
    const signer = new KmsEd25519Signer();
    const publicKey = await signer.public_key();

    expect(publicKey).toBe(mockPublicKeyPem);
  });

  test('throws descriptive error when KMS API fails', async () => {
    const mockError = new Error('KMS API error: AccessDenied');
    const mockSend = jest.fn().mockRejectedValue(mockError);

    (KMSClient as jest.MockedClass<typeof KMSClient>).mockImplementation(
      () => ({
        send: mockSend,
      } as any)
    );

    const signer = new KmsEd25519Signer();
    const payload = Buffer.from('test payload');

    await expect(signer.sign(payload)).rejects.toThrow(
      'kms signing failed: KMS API error: AccessDenied'
    );
  });

  test('throws error when KMS response missing signature', async () => {
    const mockSend = jest.fn().mockResolvedValue({
      Signature: undefined,
    });

    (KMSClient as jest.MockedClass<typeof KMSClient>).mockImplementation(
      () => ({
        send: mockSend,
      } as any)
    );

    const signer = new KmsEd25519Signer();
    const payload = Buffer.from('test payload');

    await expect(signer.sign(payload)).rejects.toThrow(
      'kms signing failed: KMS Sign response missing signature'
    );
  });

  test('integrates with AuditLogger correctly', async () => {
    const signer = new KmsEd25519Signer();
    const logger = new AuditLogger(signer, 'kms');

    const traceData = {
      input: { amount: 100, currency: 'USD' },
      state: { balance: 900 },
      events: ['TRANSFER_INIT', 'DEBIT'],
      timestamp: new Date().toISOString(),
    };

    const log = await logger.generateLog(traceData as any);

    expect(log.signature).toBeDefined();
    expect(log.publicKey).toBe(mockPublicKeyPem);
    expect(log.algorithm).toBe('Ed25519+SHA256');
    expect(log.signer.provider).toBe('kms');
  });
});
