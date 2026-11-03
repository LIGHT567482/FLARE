CXX ?= g++
CXXFLAGS ?= -fPIC -shared -std=c++17 -O2 -Wall
LDLIBS ?= -lcrypto
PREFIX ?= /usr/local
INCLUDES = -Iinclude

TARGET = libflare-crypto.so

$(TARGET): src/flare_crypto.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -o $(TARGET) src/flare_crypto.cpp $(LDLIBS)

.PHONY: clean install

clean:
	rm -f $(TARGET)

install: $(TARGET)
	install -d $(PREFIX)/lib $(PREFIX)/include
	install -m 755 $(TARGET) $(PREFIX)/lib/
	install -m 644 include/flare_crypto.h $(PREFIX)/include/

test: $(TARGET)
	$(CXX) -std=c++17 -Wall $(INCLUDES) -o /tmp/flare-crypto-test tests/crypto_test.cpp src/flare_crypto.cpp $(LDLIBS)
	/tmp/flare-crypto-test
	rm -f /tmp/flare-crypto-test
