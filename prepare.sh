#!/bin/bash

export PROTO_IMAGE=/Users/thibaultjeandet/proton/buf-lib-poc/root_namespace_buf_image.json.gz

PRIVATE_KEY="ecdsa_priv.der"
PUBLIC_KEY="ecdsa_pub.der"
CANTON_INTERMEDIATE_KEY="intermediate_pub.canton"

# 1. Generate keys
openssl ecparam -name prime256v1 -genkey -noout -outform DER -out "$PRIVATE_KEY"
openssl ec -inform der -in "$PRIVATE_KEY" -pubout -outform der -out "$PUBLIC_KEY" 2> /dev/null

# 2. Get signer fingerprint
FINGERPRINT=$(./bin/proton fingerprint "$PUBLIC_KEY")
echo "Signer Fingerprint: $FINGERPRINT"

# 3. Get intermediate key
INTERMEDIATE_KEY=$(
    ./bin/proton decode SigningPublicKey @intermediate_pub.canton --versioned | jq -r .publicKey
)

# 4. Prepare transactions
./bin/proton prepare delegation \
  --root \
  --root-key @$PUBLIC_KEY \
  --output ./root-cert

./bin/proton prepare delegation \
  --root-key @$PUBLIC_KEY \
  --target-key $INTERMEDIATE_KEY \
  --output ./intermediate-cert

# 5. Sign the hashes
openssl pkeyutl -rawin -inkey "$PRIVATE_KEY" -keyform DER -sign < "root-cert.hash" > "root-cert.sig"
openssl pkeyutl -rawin -inkey "$PRIVATE_KEY" -keyform DER -sign < "intermediate-cert.hash" > "intermediate-cert.sig"

# 6. Assemble with generic command
./bin/proton assemble \
  --prepared-transaction ./root-cert.prep \
  --signature @./root-cert.sig \
  --signature-algorithm ecdsa256 \
  --signed-by "$FINGERPRINT" \
  --output ./root-cert.cert

./bin/proton assemble \
  --prepared-transaction ./intermediate-cert.prep \
  --signature @./intermediate-cert.sig \
  --signature-algorithm ecdsa256 \
  --signed-by "$FINGERPRINT" \
  --output ./intermediate-cert.cert
