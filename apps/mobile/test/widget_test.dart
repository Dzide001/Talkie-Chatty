import 'package:flutter_test/flutter_test.dart';
import 'package:talkie_chatty/main.dart';

void main() {
  testWidgets('spike page renders stopped state', (tester) async {
    await tester.pumpWidget(const SpikeApp());
    expect(find.text('Start foreground service'), findsOneWidget);
    expect(find.textContaining('stopped'), findsOneWidget);
  });
}
