package misc_2019 is
  view v1 of entity is
    a : in;
    b : out;
  end view;

  group tmpl is (signal, variable);
  group g1 : tmpl (a, b);

  attribute foo : integer;
  attribute foo of g1 : group is 1;
end package;
