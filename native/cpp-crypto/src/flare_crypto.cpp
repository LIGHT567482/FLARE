#include "flare_crypto.h"
#include <cstdio>
#include <cstring>
#include <climits>
#include <cstdint>
#include <string>
#include <unistd.h>
#include <sys/utsname.h>
#include <openssl/evp.h>
#include <openssl/rand.h>

extern "C" {

int flare_keygen(flare_keypair_t *kp) {
    if (!kp) return -1;
    EVP_PKEY_CTX *ctx = EVP_PKEY_CTX_new_id(EVP_PKEY_ED25519, nullptr);
    EVP_PKEY *key = nullptr;
    if (!ctx) return -1;
    const bool generated = EVP_PKEY_keygen_init(ctx) == 1 &&
                           EVP_PKEY_keygen(ctx, &key) == 1;
    EVP_PKEY_CTX_free(ctx);
    if (!generated) return -1;
    size_t public_len = sizeof(kp->pk);
    size_t private_len = 32;
    const bool extracted =
        EVP_PKEY_get_raw_public_key(key, kp->pk, &public_len) == 1 &&
        EVP_PKEY_get_raw_private_key(key, kp->sk, &private_len) == 1 &&
        public_len == sizeof(kp->pk) && private_len == 32;
    EVP_PKEY_free(key);
    if (!extracted) {
        std::memset(kp, 0, sizeof(*kp));
        return -1;
    }
    std::memcpy(kp->sk + 32, kp->pk, sizeof(kp->pk));
    return 0;
}

int flare_sign(const unsigned char *msg, unsigned long long msglen,
               const unsigned char *sk, unsigned char *sig) {
    if ((!msg && msglen != 0) || !sk || !sig || msglen > SIZE_MAX) return -1;
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, nullptr, sk, 32);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    if (!key || !ctx) {
        EVP_PKEY_free(key);
        EVP_MD_CTX_free(ctx);
        return -1;
    }
    size_t siglen = 64;
    const bool signed_ok = EVP_DigestSignInit(ctx, nullptr, nullptr, nullptr, key) == 1 &&
                           EVP_DigestSign(ctx, sig, &siglen, msg, static_cast<size_t>(msglen)) == 1 &&
                           siglen == 64;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    return signed_ok ? 0 : -1;
}

int flare_verify(const unsigned char *msg, unsigned long long msglen,
                 const unsigned char *pk, const unsigned char *sig) {
    if ((!msg && msglen != 0) || !pk || !sig || msglen > SIZE_MAX) return 0;
    EVP_PKEY *key = EVP_PKEY_new_raw_public_key(EVP_PKEY_ED25519, nullptr, pk, 32);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    if (!key || !ctx) {
        EVP_PKEY_free(key);
        EVP_MD_CTX_free(ctx);
        return 0;
    }
    const bool verified = EVP_DigestVerifyInit(ctx, nullptr, nullptr, nullptr, key) == 1 &&
                          EVP_DigestVerify(ctx, sig, 64, msg, static_cast<size_t>(msglen)) == 1;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    return verified ? 1 : 0;
}

int flare_aes_gcm_encrypt(const unsigned char *key, const unsigned char *nonce,
                          const unsigned char *pt, unsigned long long ptlen,
                          unsigned char *ct, unsigned char *tag) {
    if (!key || !nonce || (!pt && ptlen != 0) || (!ct && ptlen != 0) ||
        !tag || ptlen > INT_MAX) return -1;
    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    if (!ctx) return -1;
    int written = 0;
    int final_written = 0;
    unsigned char final_buffer[16];
    const bool encrypted =
        EVP_EncryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr) == 1 &&
        EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, 12, nullptr) == 1 &&
        EVP_EncryptInit_ex(ctx, nullptr, nullptr, key, nonce) == 1 &&
        (ptlen == 0 || EVP_EncryptUpdate(ctx, ct, &written, pt, static_cast<int>(ptlen)) == 1) &&
        EVP_EncryptFinal_ex(ctx, ptlen == 0 ? final_buffer : ct + written, &final_written) == 1 &&
        EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_GET_TAG, 16, tag) == 1;
    EVP_CIPHER_CTX_free(ctx);
    return encrypted ? 0 : -1;
}

int flare_aes_gcm_decrypt(const unsigned char *key, const unsigned char *nonce,
                          const unsigned char *ct, unsigned long long ctlen,
                          const unsigned char *tag, unsigned char *pt) {
    if (!key || !nonce || (!ct && ctlen != 0) || (!pt && ctlen != 0) ||
        !tag || ctlen > INT_MAX) return -1;
    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    if (!ctx) return -1;
    int written = 0;
    int final_written = 0;
    unsigned char final_buffer[16];
    const bool decrypted =
        EVP_DecryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr) == 1 &&
        EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, 12, nullptr) == 1 &&
        EVP_DecryptInit_ex(ctx, nullptr, nullptr, key, nonce) == 1 &&
        (ctlen == 0 || EVP_DecryptUpdate(ctx, pt, &written, ct, static_cast<int>(ctlen)) == 1) &&
        EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_TAG, 16, const_cast<unsigned char *>(tag)) == 1 &&
        EVP_DecryptFinal_ex(ctx, ctlen == 0 ? final_buffer : pt + written, &final_written) == 1;
    if (!decrypted && pt && ctlen != 0) std::memset(pt, 0, static_cast<size_t>(ctlen));
    EVP_CIPHER_CTX_free(ctx);
    return decrypted ? 0 : -1;
}

int flare_hardware_id(char *out, unsigned int outlen) {
    if (!out || outlen < 1) return -1;
    struct utsname un;
    if (uname(&un) != 0) return -1;
    std::string id = std::string(un.nodename) + ":" + un.machine;
    if (id.size() >= outlen) return -1;
    strcpy(out, id.c_str());
    return 0;
}

}
