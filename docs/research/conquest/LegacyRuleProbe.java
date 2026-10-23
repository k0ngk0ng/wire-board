// Research-only probe of the official historical JAR. Not part of the game.
// Use verify_legacy_probe.py to keep JVM temporary output within the workspace.
import cg.*;
import java.util.*;
import java.lang.reflect.Constructor;

public final class LegacyRuleProbe {
  private static final class Fixture {
    final rL world = new rL();
    final fF[] players = {new fF(), new fF()};
    final bu[] regions;
    Fixture(int[] owners, int[] troops) {
      world.a(new qM());
      av setup = new av(); setup.m(true); world.a(setup);
      world.a(1); world.a(new ja()); world.a(new pJ()); world.a(new wZ());
      af entities = new af();
      for (int i = 0; i < players.length; i++) {
        fF player = players[i]; player.a("p" + i); player.a(5000L);
        player.b(32); player.c(90); player.d(10); player.e(80); player.f(80);
        player.a(new xr()); player.a(new dy()); entities.add(player);
      }
      world.a(entities); us provinces = new us(); regions = new bu[owners.length];
      for (int i = 0; i < regions.length; i++) {
        bu region = new bu(); region.a("r" + i); region.b(troops[i]);
        region.a(new us()); region.a(new ja()); region.c(25000); region.d(50000);
        if (owners[i] >= 0) {
          fF owner = players[owners[i]]; region.c(owner);
          if (owner.i() == null) owner.a(region);
        }
        regions[i] = region; provinces.add(region);
      }
      world.a(provinces); world.a(new yR()); world.a(new jX());
    }
    void link(int a, int b) {
      regions[a].l().add(regions[b]); regions[b].l().add(regions[a]);
    }
  }

  private static void battle(int sent, int defenders, boolean town, boolean fort,
                             boolean attackingRuler, boolean defendingRuler) {
    rL world = new rL();
    world.a(new qM());
    av setup = new av(); setup.m(true); world.a(setup);
    fF attacker = new fF(); attacker.a("attacker"); attacker.a(10000L);
    attacker.b(32); attacker.a(new xr());
    fF defender = new fF(); defender.a("defender"); defender.a(10000L);
    defender.b(32); defender.a(new xr());
    af entities = new af(); entities.add(attacker); entities.add(defender); world.a(entities);
    bu source = new bu(); source.a("source"); source.c(attacker); source.b(sent);
    bu target = new bu(); target.a("target"); target.c(defender); target.b(defenders);
    source.a(new us()); target.a(new us()); source.l().add(target); target.l().add(source);
    source.a(new ja()); target.a(new ja());
    target.a(town); target.c(fort);
    if (attackingRuler) attacker.a(source);
    if (defendingRuler) defender.a(target);
    us regions = new us(); regions.add(source); regions.add(target); world.a(regions);
    pA event = new pA(); event.a(0); event.b(0); event.c(1);
    event.a(attackingRuler); event.d(sent); event.b(world);
    System.out.printf("{\"sent\":%d,\"defenders\":%d,\"town\":%s,\"fort\":%s,\"attackingRuler\":%s,\"defendingRuler\":%s,\"captured\":%s,\"survivors\":%d,\"fortAfter\":%s,\"attackingRulerMoved\":%s}%n",
        sent, defenders, town, fort, attackingRuler, defendingRuler,
        target.d() == attacker, target.e(), target.h(), attacker.i() == target);
  }

  private static void capture(int remaining, long defeatedGold) {
    int[] owners = new int[remaining + 2], troops = new int[remaining + 2];
    owners[0] = 0; troops[0] = 10000;
    owners[1] = 1; troops[1] = 100;
    for (int i = 2; i < owners.length; i++) { owners[i] = 1; troops[i] = i * 101; }
    Fixture f = new Fixture(owners, troops); f.link(0, 1);
    f.players[1].a(defeatedGold);
    zF command = zF.a(f.world, f.players[0], 0, f.regions[0], f.regions[1], false, 10000);
    nh events = command.i(f.world);
    StringJoiner states = new StringJoiner(",", "[", "]");
    for (int i = 2; i < f.regions.length; i++) {
      bu region = f.regions[i];
      states.add("{\"id\":" + i + ",\"owner\":" + f.world.g().indexOf(region.d())
          + ",\"troops\":" + region.e() + "}");
    }
    bu ruler = f.players[1].i();
    System.out.printf("{\"kind\":\"capture\",\"remainingBeforeCapture\":%d,\"defeatedGoldBefore\":%d,\"alive\":%s,\"retained\":%d,\"attackerGold\":%d,\"defenderGold\":%d,\"rulerRegion\":%d,\"events\":%d,\"regions\":%s}%n",
        remaining, defeatedGold, f.players[1].n(), f.world.d(f.players[1]).size(),
        f.players[0].e(), f.players[1].e(), f.world.i().indexOf(ruler), events.size(), states);
  }

  private static void movement(String kind, int available, int requested, boolean sea,
                               boolean lostSource, boolean ruler) {
    movement(kind, available, requested, sea, lostSource, ruler, 5000L);
  }

  private static void movement(String kind, int available, int requested, boolean sea,
                               boolean lostSource, boolean ruler, long gold) {
    Fixture f = new Fixture(new int[]{0, 0, 1}, new int[]{500, 100, 100});
    f.players[0].a(gold);
    if (!sea) f.link(0, 1);
    zF command = zF.a(f.world, f.players[0], 0, f.regions[0], f.regions[1], ruler, requested);
    f.regions[0].b(available);
    if (lostSource) f.regions[0].c(f.players[1]);
    nh events = command.i(f.world);
    System.out.printf("{\"kind\":\"movement\",\"case\":\"%s\",\"available\":%d,\"requested\":%d,\"sourceAfter\":%d,\"targetAfter\":%d,\"moneyAfter\":%d,\"mp10After\":%d,\"rulerRegion\":%d,\"eventCount\":%d}%n",
        kind, available, requested, f.regions[0].e(), f.regions[1].e(), f.players[0].e(),
        f.players[0].f(), f.world.i().indexOf(f.players[0].i()), events.size());
  }

  private static void recruit(String kind, int requested, long gold, int population) {
    Fixture f = new Fixture(new int[]{0, 1}, new int[]{500, 500});
    wP command = wP.a(f.world, f.players[0], 0, f.regions[0], requested);
    f.players[0].a(gold); f.regions[0].c(population);
    nh events = command.i(f.world);
    System.out.printf("{\"kind\":\"recruit\",\"case\":\"%s\",\"requested\":%d,\"goldBefore\":%d,\"populationBefore\":%d,\"troopsAfter\":%d,\"goldAfter\":%d,\"populationAfter\":%d,\"mp10After\":%d,\"eventCount\":%d}%n",
        kind, requested, gold, population, f.regions[0].e(), f.players[0].e(),
        f.regions[0].i(), f.players[0].f(), events.size());
  }

  private static void planning(boolean ruler) {
    Fixture f = new Fixture(new int[]{0, 0, 1}, new int[]{500, 100, 100}); f.link(0, 1);
    zF command = zF.a(f.world, f.players[0], 0, f.regions[0], f.regions[1], ruler, 300);
    command.g(f.world);
    System.out.printf("{\"kind\":\"planning-projection\",\"rulerTravels\":%s,\"sourceTroops\":%d,\"targetTroops\":%d,\"rulerRegion\":%d,\"mp10\":%d}%n",
        ruler, f.regions[0].e(), f.regions[1].e(), f.world.i().indexOf(f.players[0].i()), f.players[0].f());
  }

  private static void economy(String kind, long money, int taxRate, int ecoRate,
                              int milRate, int turns, boolean linked) {
    Fixture f = new Fixture(new int[]{0, 1}, new int[]{500, 500});
    f.regions[0].a(true);
    if (linked) f.link(0, 1);
    fF player = f.players[0]; player.a(money); player.d(taxRate);
    player.e(ecoRate); player.f(milRate);
    for (int i = 0; i < turns; i++) {
      f.world.a(i + 1); new ei().b(f.world);
      fl income = new dg().a(f.world, player); new dg().b(f.world);
      System.out.printf("{\"kind\":\"economy\",\"case\":\"%s\",\"turn\":%d,\"population\":%d,\"economy\":%d,\"troops\":%d,\"morale\":%d,\"mp10\":%d,\"taxIncome\":%d,\"economySpending\":%d,\"militarySpending\":%d,\"money\":%d,\"neighborPopulation\":%d}%n",
          kind, i+1, f.regions[0].i(), f.regions[0].j(), f.regions[0].e(), player.j(),
          player.f(), income.b(), income.d(), income.e(), player.e(), f.regions[1].i());
    }
  }

  private static void order(String kind, int sequence0, int sequence1, int[] areaCounts,
                            int troops0, int troops1, long gold0, long gold1) throws Exception {
    Fixture f = new Fixture(new int[]{0, 1}, new int[]{troops0, troops1}); f.link(0, 1);
    f.players[0].a(gold0); f.players[1].a(gold1);
    zF first = zF.a(f.world, f.players[0], sequence0, f.regions[0], f.regions[1], false, 1);
    zF second = zF.a(f.world, f.players[1], sequence1, f.regions[1], f.regions[0], false, 1);
    Constructor<?> ctor = Class.forName("cg.ev").getDeclaredConstructors()[0]; ctor.setAccessible(true);
    Comparator comparator = (Comparator)ctor.newInstance(f.world,
        Arrays.asList(areaCounts[0], areaCounts[1]), f.world.g());
    int compared = comparator.compare(first, second);
    wP recruit = wP.a(f.world, f.players[1], 99, f.regions[1], 1);
    mt disband = mt.a(f.world, f.players[1], 99, f.regions[1], 1);
    System.out.printf("{\"kind\":\"order\",\"case\":\"%s\",\"firstPlayer\":%d,\"recruitBeforeMove\":%s,\"recruitBeforeDisband\":%s,\"disbandBeforeMove\":%s}%n",
        kind, compared < 0 ? 0 : 1, comparator.compare(recruit, first) < 0,
        comparator.compare(recruit, disband) < 0, comparator.compare(disband, first) < 0);
  }

  public static void main(String[] args) throws Exception {
    qM market = new qM();
    System.out.printf("{\"kind\":\"market\",\"recruit1000\":%d,\"shipping1000\":%d,\"fort\":%d,\"tower\":%d,\"town\":%d,\"upkeep3500at80\":%d,\"tax25000at10and90\":%d}%n",
        market.e(1000), market.d(1000), market.a(), market.b(), market.c(),
        market.b(3500L, 80), market.a(25000L, 10, 90));
    battle(1000, 500, false, false, false, false);
    battle(1000, 1000, false, false, false, false);
    battle(1000, 1000, false, true, false, false);
    battle(4000, 1000, true, true, false, true);
    battle(1000, 2000, false, false, true, false);
    battle(1000, 500, false, true, true, false);
    battle(1000, 0, false, true, false, false);
    for (int remaining : new int[]{0, 1, 2, 3, 5, 6, 11, 12, 17}) capture(remaining, 1234L);
    capture(3, -100L);
    movement("land-ruler", 500, 300, false, false, true);
    movement("sea", 500, 300, true, false, false);
    movement("reduced-before-execution", 200, 500, false, false, false);
    movement("sea-reduced-before-execution", 200, 500, true, false, false);
    movement("source-conquered", 500, 300, false, true, false);
    movement("sea-cannot-afford-request", 200, 500, true, false, false, 500L);
    movement("source-empty", 0, 300, false, false, false);
    recruit("affordable", 1000, 1000L, 25000);
    recruit("gold-lost-before-execution", 1000, 500L, 25000);
    recruit("population-lost-before-execution", 1000, 5000L, 1000);
    planning(false); planning(true);
    economy("default-town", 5000L, 10, 80, 80, 12, false);
    economy("bankrupt", -10000L, 10, 80, 80, 1, false);
    economy("recovered-debt", -100L, 10, 80, 80, 1, false);
    economy("high-tax", 5000L, 20, 80, 80, 1, false);
    economy("high-military", 5000L, 10, 80, 100, 1, false);
    economy("migration", 5000L, 10, 80, 80, 1, true);
    order("sequence", 0, 1, new int[]{1, 2}, 100, 200, 10, 20);
    order("territories", 0, 0, new int[]{1, 2}, 1000, 200, 100, 20);
    order("military", 0, 0, new int[]{1, 1}, 100, 200, 100, 20);
    order("gold", 0, 0, new int[]{1, 1}, 100, 100, 10, 20);
    order("seat", 0, 0, new int[]{1, 1}, 100, 100, 10, 10);
  }
}
