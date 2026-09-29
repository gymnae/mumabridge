# Mumabridge

A Go service for a one-to-one, bidirectional voice bridge between a Mumble channel and an Element Call MatrixRTC room backed by LiveKit.

## Identity and media model

- Every real Mumble user is represented by a namespaced Matrix application-service ghost and a distinct LiveKit participant/audio track.
- Every real MatrixRTC participant is represented by a distinct Mumble client ghost.
- Audio remains separated by speaker; it is never mixed into a single bridge user.
- The internal media format is signed 16-bit, 48 kHz, mono PCM in 20 ms frames.
- Matrix ghosts use the exclusive `mumabridge_mumble_` namespace. Mumble ghosts use the `[mx] ` prefix. LiveKit ghosts carry `{"mumabridge":"true"}` metadata. These markers are excluded from reconciliation to prevent recursive ghosts and feedback loops.

## Direct LiveKit mode

No focus/JWT service URL is required. The bridge reads the LiveKit URL, API key, and API secret from environment variables and signs short-lived participant JWTs locally. `LIVEKIT_ROOM` defaults to `MATRIX_ROOM_ID`; set it explicitly only when the Element Call deployment maps the Matrix room to a different LiveKit room name.

The API secret is highly privileged. Keep it in a container secret or protected environment file and never expose it to a browser.

## Configuration

Copy `.env.example` to `.env` and set at least:

- `MATRIX_HOMESERVER_URL`
- `MATRIX_SERVER_NAME`
- `MATRIX_ROOM_ID`
- `MATRIX_AS_TOKEN`
- `MATRIX_HS_TOKEN`
- `LIVEKIT_URL`
- `LIVEKIT_API_KEY`
- `LIVEKIT_API_SECRET`
- `MUMBLE_ADDRESS`

All deployment values are environment variables. The process deliberately does not log its complete configuration because it contains secrets.

## Matrix application service setup

1. Copy `config/appservice-registration.yaml` to the homeserver.
2. Generate separate high-entropy values for `as_token` and `hs_token` and place the same values in the service environment.
3. Replace `example.org` in the exclusive user namespace with the escaped Matrix server name.
4. Register the file using the homeserver's application-service mechanism and restart the homeserver.
5. Ensure the application-service URL is reachable by the homeserver at port 8080.

The application service can only control users in its exclusive ghost namespace. Its sender and ghost users must be allowed to join the configured room.

## Container operation

```sh
cp .env.example .env
# edit .env and the application-service registration
docker compose up --build -d
```

Endpoints:

- `GET /healthz`: process liveness
- `GET /readyz`: bridge readiness
- `PUT /_matrix/app/v1/transactions/{transactionId}`: Matrix application-service transactions

The image is multi-stage, contains no secrets, and runs as a non-root distroless user.

## Development

Go 1.25 or newer plus development packages for Opus and libsoxr are required. The container installs these automatically.

```sh
make fmt
make test
make vet
make build
```

## Migration from the previous name

This revision changes the project, executable, container service, Matrix application-service sender and exclusive ghost namespace, LiveKit marker, and default state directory from the former misspelling to `mumabridge`. Replace the homeserver application-service registration, move any persistent data to `/var/lib/mumabridge`, and remove or explicitly retire ghosts created in the old namespace before rollout. Running both registrations against the same room can create duplicate ghosts.

## Current implementation boundary

The supervisor now serializes Matrix membership and reconnecting Mumble membership snapshots, creates opposite-side ghosts, routes per-speaker PCM, rejects stale connection generations, and drives readiness from initial source snapshots. Reconnect retries use capped exponential backoff with jitter and shutdown removes managed ghosts deterministically.

The deployability boundary remains staging evidence: validate the exact Element Call-to-LiveKit room mapping and complete every acceptance scenario below against the target services. Do not declare this revision production-ready until those results are recorded. LiveKit membership is represented by MatrixRTC membership events; deployments whose MatrixRTC membership model differs require an adapter adjustment discovered during staging.

## MVP acceptance test

Against a staging Matrix/Element Call, LiveKit, and Mumble deployment:

1. Join Mumble as Arnold; confirm one Matrix room member and LiveKit participant named Arnold appears.
2. Join Element Call as Beatrice; confirm one Mumble user prefixed `[mx] Beatrice` appears.
3. Speak on each side and verify intelligible audio on the other side under the matching ghost.
4. Confirm bridge-owned users do not create additional ghosts and no returned audio loop develops.
5. Leave each side and confirm the corresponding ghost disappears.
6. Restart the bridge and each endpoint independently; confirm `/readyz` becomes unavailable during lost source state and returns after reconciliation.
7. Interrupt Mumble and LiveKit network access long enough to trigger retries; confirm reconnection creates no duplicate or orphaned ghosts.
8. Record the resolved Element Call-to-LiveKit room name, timestamps, bridge logs, participant lists, and pass/fail result for every step before deployment approval.
