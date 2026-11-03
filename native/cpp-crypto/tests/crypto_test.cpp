#include "flare_crypto.h"
#include <array>
#include <cassert>
#include <cstring>

int main() {
    flare_keypair_t keypair{};
    assert(flare_keygen(&keypair) == 0);

    constexpr unsigned char message[] = "FLARE authenticated message";
    std::array<unsigned char, 64> signature{};
    assert(flare_sign(message, sizeof(message), keypair.sk, signature.data()) == 0);
    assert(flare_verify(message, sizeof(message), keypair.pk, signature.data()) == 1);
    std::array<unsigned char, sizeof(message)> tampered_message{};
    std::memcpy(tampered_message.data(), message, sizeof(message));
    tampered_message[0] ^= 1;
    assert(flare_verify(tampered_message.data(), tampered_message.size(), keypair.pk, signature.data()) == 0);

    std::array<unsigned char, 32> aes_key{};
    std::array<unsigned char, 12> nonce{};
    constexpr unsigned char plaintext[] = "offline data";
    std::array<unsigned char, sizeof(plaintext)> ciphertext{};
    std::array<unsigned char, 16> tag{};
    assert(flare_aes_gcm_encrypt(aes_key.data(), nonce.data(), plaintext,
                                 sizeof(plaintext), ciphertext.data(), tag.data()) == 0);

    std::array<unsigned char, sizeof(plaintext)> decrypted{};
    assert(flare_aes_gcm_decrypt(aes_key.data(), nonce.data(), ciphertext.data(),
                                 ciphertext.size(), tag.data(), decrypted.data()) == 0);
    assert(std::memcmp(plaintext, decrypted.data(), sizeof(plaintext)) == 0);

    tag[0] ^= 1;
    decrypted.fill(0xFF);
    assert(flare_aes_gcm_decrypt(aes_key.data(), nonce.data(), ciphertext.data(),
                                 ciphertext.size(), tag.data(), decrypted.data()) != 0);
    for (unsigned char byte : decrypted) assert(byte == 0);
    return 0;
}
