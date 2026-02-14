# Robot FPGA + Linux "Brain" Architecture Notes (Draft)

## What You Want (My Understanding)

You want to build a robotics stack where:

- A Linux "brain" focuses on high-level behavior and control decisions (planning, perception, task logic).
- An FPGA sits closer to the hardware and "owns timing" so Linux does not need to manage microsecond-level scheduling.
- The FPGA directly interfaces with many sensors and actuators, packages sensor data into consistent frames, and applies actuator commands at defined times.
- You want to avoid vendor lock-in and proprietary ecosystems as much as possible.
- You do not care if the robot is fast; you care that it is capable (house mobility, chores, stairs), takes in video, and controls motors/actuators.

## Conceptual Block Diagram

- Linux brain (Ethernet/USB/etc.): perception (video), planning (navigation/stairs), supervisory control, talks to FPGA using framed telemetry/commands.
- FPGA (hard real-time boundary): sensor sampling + timestamping, signal conditioning, actuator output generation (PWM, step/dir), safety/watchdogs, communication endpoint (EtherCAT slave, CAN, RS-485, Ethernet UDP/TCP).
- Power + analog hardware (physical layer): motor power stages (bridges, gate drivers, current sense, protection), ADCs, level shifting, isolation where needed.

## "FPGA Owns Timing" (The Contract You’re Aiming For)

The key idea is to define a simple, strict contract between Linux and the FPGA:

- The FPGA runs one or more fixed-rate ticks (examples: 250 Hz control tick, 10 kHz PWM carrier, 100 Hz slow sensors).
- On each control tick, the FPGA produces one coherent `SensorFrame` snapshot.
- `SensorFrame` includes: a timestamp or `tick_id`, all sensor values sampled in a known window, validity bits, fault/status flags.
- Linux produces `ActuatorCmd` frames.
- `ActuatorCmd` includes: `seq`, a target `tick_id` (or "apply on next tick"), setpoints, mode bits, a watchdog expectation (implicit or explicit).
- The FPGA applies commands only at the defined boundary (tick edge), and trips to a safe state if commands are stale or invalid.

This hides Linux scheduling jitter behind buffering, sequence numbers, and deadlines.

## "Protocol" (What That Means Here)

When we say "Ethernet + UDP/TCP + your own protocol", "protocol" means your application-level rules on top of UDP/TCP:

- Message types (examples: `SENSOR_FRAME`, `ACTUATOR_CMD`, `HEARTBEAT`, `CONFIG_READ`, `CONFIG_WRITE`)
- Binary layouts (field sizes, endianness, signed vs unsigned)
- Units/scaling (fixed-point formats, calibration)
- Robustness rules (sequence numbers, CRC, timeouts, ack/retry policy)
- Versioning rules (how you evolve formats without breaking older software)

The goal is for Linux to treat the FPGA like a deterministic IO appliance.

## Buses/Links We Discussed (Tradeoffs)

### EtherCAT (Industrial Real-Time Ethernet)

What attracted you:

- Deterministic cyclic exchange and sync concepts.
- It "feels like" a hardware-timed IO plane.

Reality constraints:

- In practice, an EtherCAT slave usually uses an EtherCAT Slave Controller (ESC) chip/module.
- EtherCAT is widely standardized, but it is not the same thing as "fully open specs freely available"; it is an ecosystem with membership/spec access norms.

### CAN (Classic / CAN-FD)

Why it’s appealing:

- Very common in robotics for distributed devices, robust wiring, simple systems.
- Multi-vendor, lots of parts, lots of tooling.

Limitations:

- Much lower bandwidth than Ethernet-class links.
- Timing is priority/arbitration based; you can design for good behavior, but it is not "one deterministic cyclic frame" in the EtherCAT sense.

### "Truly Open-ish" Options

If you want maximum vendor neutrality and low lock-in:

- Plain Ethernet with UDP/TCP + your protocol: pros are ubiquitous/multi-vendor/easy Linux integration; cons are you must design the real-time contract (ticks, buffering, watchdog).
- RS-485 with a simple framed protocol (or Modbus RTU): pros are robust over longer cables/simple/cheap; cons are lower bandwidth and it fits "slow but reliable" control/telemetry better than high-rate loops.

## The "Directly Talk to Motors" Point (Important Clarification)

You can absolutely have the FPGA be the only "digital controller" that generates control signals and reads sensors, but:

- A motor always needs power electronics (drivers, bridges, current sensing, protection).
- For BLDC/PMSM motors, if you want the FPGA to do full servo-drive behavior (FOC current loops at tens of kHz), that is feasible but a big project.
- If you pick actuator types with simpler command interfaces (step/dir, PWM + direction, analog setpoint), the FPGA part is much simpler and more vendor-neutral.

## Where You Seem Confused (Or Where the Terms Collide)

- "Avoid proprietary anything" vs "avoid lock-in": avoiding lock-in is realistic (standard links + clean internal interfaces); avoiding all proprietary components is not realistic in hardware (silicon is proprietary, many standards specs are paywalled, vendor register maps exist).
- "FPGA directly drives motors" ambiguity: "FPGA generates PWM/step-dir and reads encoders" is straightforward; "FPGA is the full servo drive (FOC/current control/protections)" is much harder.
- EtherCAT motivation vs requirements: EtherCAT matches "deterministic cyclic IO", but your stated priority is lock-in avoidance and you don’t need speed, which often points toward Ethernet/UDP or RS-485 with a strong FPGA timing contract.

## Open Questions (Need Answers to Nail the Architecture)

- Actuators: motor types (steppers, brushed DC, BLDC/PMSM, hobby servos, linear actuators, valves), rough power levels (voltage/current), axis count.
- Control expectations: acceptable update rate (50 Hz, 100 Hz, 250 Hz, 1 kHz), control mode needs (torque, velocity, position-ish).
- Topology: one central FPGA board vs distributed IO boards, cable lengths, EMI environment.
- Sensors: main categories (IMU, encoders, force/torque, proximity, limit switches, pressure, temperature, cameras), which are on-board vs remote.
- Video: whether video ever touches FPGA vs direct to Linux via USB/Ethernet (recommendation: keep video off FPGA unless there is a strong reason).
- Safety: safe state per actuator on comms loss, whether E-stop bypasses Linux, what faults must be handled in hardware.
- "Openness" requirement details: optimizing for multi-vendor swappability (interfaces) vs freely downloadable specs vs both.

## Suggested Next Step (Conversation-Level, Not Implementation)

Pick one of these and we can flesh it out:

- Option A: Ethernet + UDP + custom framed protocol (FPGA timing contract, watchdog, CRC, seq/tick IDs).
- Option B: RS-485 + simple framed protocol (slower, rugged, very lock-in resistant).
- Option C: CAN (prioritized messages + fixed rate scheduling, still simple, lots of robotics precedent).

Once the link is chosen, the next design anchor is the `SensorFrame` and `ActuatorCmd` format and the timing/watchdog contract.
