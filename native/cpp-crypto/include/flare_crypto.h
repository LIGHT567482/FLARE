#pragma once

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    unsigned char pk[32];
    unsigned char sk[64];
} flare_keypair_t;

// Ed25519 secret keys are stored as the 32-byte seed followed by the 32-byte
// public key. AES-GCM uses a 12-byte nonce and a 16-byte authentication tag.
int flare_keygen(flare_keypair_t *kp);
int flare_sign(const unsigned char *msg, unsigned long long msglen,
               const unsigned char *sk, unsigned char *sig);
int flare_verify(const unsigned char *msg, unsigned long long msglen,
                 const unsigned char *pk, const unsigned char *sig);

int flare_aes_gcm_encrypt(const unsigned char *key, const unsigned char *nonce,
                          const unsigned char *pt, unsigned long long ptlen,
                          unsigned char *ct, unsigned char *tag);
int flare_aes_gcm_decrypt(const unsigned char *key, const unsigned char *nonce,
                          const unsigned char *ct, unsigned long long ctlen,
                          const unsigned char *tag, unsigned char *pt);

// Hardware fingerprint
int flare_hardware_id(char *out, unsigned int outlen);

#ifdef __cplusplus
}
#endif
