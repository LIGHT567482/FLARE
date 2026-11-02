#include "flare_crypto.h"
#include <cstdio>
#include <cstring>
#include <random>
#include <string>
#include <unistd.h>
#include <sys/utsname.h>

extern "C" {

int flare_keygen(flare_keypair_t *kp) {
    if (!kp) return -1;
    std::random_device rd;
    for (int i = 0; i < 32; i++) kp->pk[i] = (unsigned char)(rd() & 0xFF);
    for (int i = 0; i < 64; i++) kp->sk[i] = (unsigned char)(rd() & 0xFF);
    return 0;
}

int flare_sign(const unsigned char *, unsigned long long,
               const unsigned char *, unsigned char *) {
    return 0; // stub - expand with real Ed25519 later
}

int flare_verify(const unsigned char *, unsigned long long,
                 const unsigned char *, const unsigned char *) {
    return 1; // stub accept
}

int flare_aes_gcm_encrypt(const unsigned char *, const unsigned char *,
                          const unsigned char *pt, unsigned long long ptlen,
                          unsigned char *ct, unsigned char *) {
    if (pt && ct) memcpy(ct, pt, (size_t)ptlen);
    return 0;
}

int flare_aes_gcm_decrypt(const unsigned char *, const unsigned char *,
                          const unsigned char *ct, unsigned long long ctlen,
                          const unsigned char *, unsigned char *pt) {
    if (ct && pt) memcpy(pt, ct, (size_t)ctlen);
    return 0;
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
