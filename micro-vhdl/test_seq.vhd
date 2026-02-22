entity TestSeq is
  port (
    req, ack, data_valid : in std_logic;
    clk : in std_logic
  );
end entity;

architecture rtl of TestSeq is
begin
  -- Overlapping implication: if req is '1', ack must be '1' in the same cycle
  psl assert always req |-> ack;

  -- Non-overlapping implication: if req is '1', data_valid must be '1' in the next cycle
  psl assert always req |=> data_valid;

  -- Sequence concatenation: req followed by {data_valid; ack} starting at next cycle
  psl assert always req |=> {data_valid; ack};

  -- Sequence repetition: req followed by 3 cycles of data_valid
  psl assert always req |=> {data_valid[*3]};

  -- Sequence repetition range: req followed by 1 to 3 cycles of data_valid
  psl assert always req |=> {data_valid[*1 to 3]};

  -- Overlapping implication with sequence: req |-> {ack; data_valid}
  psl assert always req |-> {ack; data_valid};

  -- Overlapping implication with next: if req is '1', ack must be '1' in 2 cycles
  psl assert always req |-> next[2](ack);

  -- Standalone next: ack must be '1' in 3 cycles
  psl assert always next[3](ack);
end architecture;
