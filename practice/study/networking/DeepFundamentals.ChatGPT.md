# Deep Networking Fundamentals

## Twisted pair
- Twisting is necessary because of wire signal interference, which can lead to degradation
- Used in ethernet, where the shared noise compared between a pair of wires by a receiver can be cancelled out.
- At higher speeds, frequencies, distances signal quality can degrade
- Defined by TIA/EIA cabling standards
- Uses differential signalling
    - The receiver compares voltage difference between two wires
    - Shared noise cancels out => improves signal integrity

- The better the grade of cable, the better the 
    - shielding
    - usable frequency range
    - crosstalk resistance (Resist unwanted signal induction via electromagnetic inference between communciation channel in a pair of wires)
        - Crosstalk: interference between pairs within the same cable
        - Alien crosstalk: interference from neighbouring cables
    - insulation
    - twist quality

### Breakdown
The “Cat” = Category, and each version defines:

- max speed (bandwidth)
- max frequency (MHz)
- shielding quality (interference resistance)
- max reliable distance


Cat5e (baseline, still everywhere)
- 1 Gbps
- 100 meters
- ~100 MHz
- Cheap, good enough for home / basic office

Reality: Most homes still run on this

Cat6 (better signal quality)
- 1 Gbps (full distance)
- 10 Gbps (short distances ~55m)
- ~250 MHz
- Tighter twists => less crosstalk


Cat6a (proper 10 Gbps cable)
- 10 Gbps
- 100 meters
- ~500 MHz
- Exists because Cat6 cannot reliably support 10 Gbps at full 100m distance
- Better shielding (reduces alien crosstalk = interference from nearby cables)
- Used in offices

Cat8 (data centre only, don’t overthink it)
- 25–40 Gbps
- Only up to ~30 meters
- ~2000 MHz
- Heavy shielding
- Not part of standard building cabling (not used for horizontal cabling)
- Primarily short-range data centre interconnects
- Used in racks, not buildings

- Cat5e: basic, good enough for 1G
- Cat6: cleaner, supports more frequency, 10G only short distance
- Cat6a: proper 10G at full building distance
- Cat8: short-range, very high-speed, data-centre niche

### Trade-offs

Better cable grade usually means:
- higher performance
- higher cost
- increased stiffness 
- more awkward installation

Twist grades
- Cat5e = loose twist
- Cat6 = tighter
- Cat6a = tighter + shielding
- Cat8 = very tight + heavy shielding

## Single-mode vs Multi-mode Fibre
- Mode determines the path of traversal
- Single-mode uses a narrow core for a single light path => long distance. Multi-mode uses a wider core with multiple paths => cheaper but limited distance due to dispersion.
    - Single-mode: Long-distance (km) => ISP backbone, metro, WAN links
    - Multi-mode: Short-distance (100–400m) => Data centres, buildings, campus networks

## Single-mode Fiber
- Uses wavelengths: 1310 nm, 1550 nm
- ~ 9 micrometers in core size
- 1 path with straight line traversal and minimal signal distortion
- Travels kilometres (long-distance transmission)
- low dispersion with light source as Laser
- Uses:
    - ISP backbone
    - long-distance links

## Multi-mode Fiber
- Uses wavelengths:  850 nm, 1300 nm
- 50-62 micrometers (5-7x core size of single mode fibre)
- Light bounces in multiple paths
- distances tend to be short, typically 100–400m (can extend up to ~600m depending on OM type and speed)
- high dispersion, characterised via modal dispersion 
    - Modal Dispersion
        - where light rays take different paths causing them to arrive at the receiver at different times
        - causes pulse broadening and inter-symbol interference which limits bandwidth and transmission speed of fiber
    - Bandwidth: 
        - Internet connection capacity
        - Max data transfer rate across a network path in a given amount of time. measured in GBps, MBps
    - Transmission speed (Throughput):
        - Actual rate at whcih data travels
        - Determines how fast data moves through pipe
        - same unit of measurement as bandwidth
    - Bandwidth is capacity (how much), speed is performance (how fast).
    - Goal: High bandwidth ensures many users can stream, game, and browse at the same time without slowing down the network, 
    while high speed makes downloads and uploads happen faster.
    - Pulse
        - A short burst of signal representing data (light/electric)
        - At high speeds, pulses becomes shorter and closer
    - Dispersion causes pulses to spread => leads to inter-symbol interference => limits bandwidth and transmission speed

- Uses LED/VCSEL
    - VCSEL:  
        - Vertical-Cavity Surface-Emitting Laser, 
        - emits light vertically from top of surface
        - Traditional lasers like that in single-mode fibre emit from edge
        - A VCSEL consists of 
            - an active region (where light is generated) 
            - sandwiched between two highly reflective Distributed Bragg Reflector (DBR) mirrors. 
            - These mirrors create a vertical cavity that causes the light to oscillate and emit perpendicularly to the wafer layers. 
- Multi-mode uses OM designations
    - OM => Optical Multi-mode
    - Standard classification for multi-mode fiber defined by ISO/IEC, which defines
        - fibre quality
        - usable bandwidth
        - performance at different speeds/distances
    - OM ratings 
        - answer "How well does the fibre control dispersion at high speeds?"
        - Rated using Bandwidth-distance product (MHz-km)
        - checks how much signal can be transmitted over a given distance before it becomes unusable
    - Ratings summary
        - OM1/OM2 => legacy, poor for high speed
        - OM3 => baseline for modern high-speed MMF
        - OM4 => improved, more distance at same speeds
        - OM5 => specialised multi-wavelength optimisation
    - Types
        - OM1:
            - Core is 62 micrometers
            - Poor for modern speeds
            - obsolete
            - 1G =>275m, 10G => 33m
        - OM2: 
            - Core is 50 micrometers
            - Designed for laser sources like VCSEL
            - designed to be transitional to OM3
            - rarely used
        - OM3: 
            - core is 50 micrometers
            - modernised OM3, optimised for laser
            - Approximate speeds: 1G => 1000m, 10G => 300m, 40G => 100m, 100G => 70m
            - Usable multi-mode standard
        - OM4: 
            - Higher bandwidth than OM3
            - Improved dispersion control
            - Approximate speeds: 1G => +100m, 10G => +100m, 40G => +50m, 100G => +30m
            - Standard in modern data centres
        - OM5:
            - Designed for SWDM (short wave wavelength division multiplexing)
            - multi-wavelength support for the same fibre
    - As OM increases
        - Modal dispersion reduces
        - Effective bandwidth increases
        - distance supported at high speed increases
        - More controlled light injection
        - Pulses get shorter
        - Signal integrity can be mantained
        
