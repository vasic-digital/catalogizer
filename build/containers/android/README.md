# IMG-ANDROID

Gradle and Android SDK build image: JDK 21, command-line tools build 11076708 (SHA-256 verified), platform-tools, build-tools 34.0.0 and 35.0.0, platforms android-34 and 35. Consolidates `docker/Dockerfile.android4` and the Android part of `docker/Dockerfile.builder` (docs/10 W10-01); the old variants stay in place until this image is proven, their removal is an owner question (11.4.122). The repository's Gradle wrapper is used, not a host Gradle.
Authored by T106; the first remote build and the lock entry are T144's (WP-14). Not built here: the Ubuntu snapshot service URL and the SDK package names are UNCONFIRMED until that build. Google's page now lists command-line tools build 15859902; moving to it is a separate reviewed change.
